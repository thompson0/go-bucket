package buckets

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type TakeoverResult struct {
	Domain     string
	CNAME      string
	Provider   Provider
	BucketName string
	Vulnerable bool
	Reason     string
	StatusCode int
}

// DetectTakeover verifica se um domínio/bucket pode ser reivindicado
// (DNS apontando para storage, mas o recurso não existe).
func DetectTakeover(target string, provider Provider, debug bool) TakeoverResult {
	res := TakeoverResult{
		Domain:     strings.TrimSpace(target),
		Provider:   provider,
		Vulnerable: false,
	}

	domain := ResourceName(target, provider)

	// Alvo informado já como hostname de storage (ex: foo.s3.amazonaws.com)
	explicitStorageHost := containsStorageSuffix(target)

	// 1. Chase do CNAME para detectar o provedor de storage
	if cname, ok := chaseCNAME(domain); ok {
		res.CNAME = cname

		if prov, ok := storageProviderFromCNAME(cname); ok {
			res.Provider = prov
			res.BucketName = bucketNameFromCNAME(domain, cname, prov)
		} else {
			res.Reason = "CNAME não aponta para storage"
			return res
		}
	} else {
		// 2. Fallback: detecção via HTTP headers
		dnsResult := DnsResolver(domain, debug)
		if dnsResult.IsReachable {
			res.Provider = Provider(dnsResult.Provider)
			res.BucketName = ResourceName(domain, res.Provider)
		} else if explicitStorageHost || !strings.Contains(domain, ".") {
			// Nome de bucket puro ou hostname de storage informado explicitamente:
			// sem CNAME/HTTP, mas o provedor é conhecido — probe direto no storage.
			res.BucketName = ResourceName(domain, provider)
		} else {
			res.Reason = dnsResult.Error
			if res.Reason == "" {
				res.Reason = "Sem CNAME nem resposta de storage"
			}
			return res
		}
	}

	if res.BucketName == "" {
		res.BucketName = ResourceName(domain, res.Provider)
	}

	// 3. Valida o nome do bucket candidato
	if !isValidBucketName(res.BucketName) {
		res.Reason = "Nome não é um nome de bucket válido"
		return res
	}

	// 4. Probe: o recurso realmente existe no provedor?
	status, body := probeStorage(res.BucketName, res.Provider, debug)
	res.StatusCode = status

	switch status {
	case 200:
		res.Reason = "Bucket existe e está respondendo"
	case 403:
		res.Reason = "Bucket existe (acesso restrito)"
	case 301, 307, 308:
		res.Reason = "Bucket existe (redirecionamento de região)"
	case 404:
		res.Vulnerable = true
		res.Reason = "DNS apontando para storage mas recurso não existe (pode ser reivindicado)"
	default:
		if strings.Contains(body, "NoSuchBucket") ||
			strings.Contains(body, "StorageNotFound") ||
			strings.Contains(body, "NoSuchAccount") ||
			strings.Contains(body, "ContainerNotFound") {
			res.Vulnerable = true
			res.Reason = "DNS apontando para storage mas recurso não existe (pode ser reivindicado)"
		} else if status == 0 {
			res.Reason = "Inconclusivo (falha de rede)"
		} else {
			res.Reason = fmt.Sprintf("Inconclusivo (HTTP %d)", status)
		}
	}

	return res
}

// containsStorageSuffix verifica se o alvo já é um hostname de storage conhecido.
func containsStorageSuffix(target string) bool {
	t := strings.ToLower(strings.TrimSpace(target))
	suffixes := []string{
		"s3.amazonaws.com",
		"s3-website",
		"storage.googleapis.com",
		"commondatastorage.googleapis.com",
		"blob.core.windows.net",
		"azureedge.net",
	}
	for _, s := range suffixes {
		if strings.Contains(t, s) {
			return true
		}
	}
	return false
}

// chaseCNAME segue a cadeia de CNAMEs (até 5 hops) e retorna o destino final.
// O bool indica se um CNAME real foi encontrado.
func chaseCNAME(domain string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	current := strings.ToLower(strings.TrimSpace(domain))
	current = strings.TrimSuffix(current, ".")

	for i := 0; i < 5; i++ {
		target, err := net.DefaultResolver.LookupCNAME(ctx, current)
		if err != nil {
			return "", false
		}

		target = strings.ToLower(strings.TrimSpace(target))
		target = strings.TrimSuffix(target, ".")

		if target == "" || target == current {
			if i == 0 {
				return "", false
			}
			return current, true
		}

		current = target
	}

	return current, true
}

// bucketNameFromCNAME deriva o nome do bucket a partir do destino do CNAME.
func bucketNameFromCNAME(domain, cname string, prov Provider) string {
	cname = strings.ToLower(cname)

	switch prov {
	case ProviderAWS:
		// ex: bucket.s3.amazonaws.com, bucket.s3.us-west-2.amazonaws.com,
		//     bucket.s3.dualstack.us-east-1.amazonaws.com, bucket.s3-website-us-east-1.amazonaws.com
		if idx := strings.Index(cname, ".s3."); idx != -1 {
			return cname[:idx]
		}
		if idx := strings.Index(cname, ".s3-website"); idx != -1 {
			return cname[:idx]
		}
		if cname == "s3.amazonaws.com" || strings.HasPrefix(cname, "s3.") {
			return ResourceName(domain, ProviderAWS)
		}
		return strings.SplitN(cname, ".", 2)[0]
	case ProviderAzure:
		// ex: bucket.blob.core.windows.net, bucket.azureedge.net
		if idx := strings.Index(cname, ".blob.core.windows.net"); idx != -1 {
			return cname[:idx]
		}
		if idx := strings.Index(cname, ".azureedge.net"); idx != -1 {
			return cname[:idx]
		}
		return strings.SplitN(cname, ".", 2)[0]
	case ProviderGCP:
		// ex: bucket.storage.googleapis.com, c.storage.googleapis.com (wildcard)
		if cname == "c.storage.googleapis.com" {
			return ResourceName(domain, ProviderGCP)
		}
		if idx := strings.Index(cname, ".storage.googleapis.com"); idx != -1 {
			return cname[:idx]
		}
		return strings.SplitN(cname, ".", 2)[0]
	}

	return ResourceName(domain, prov)
}

// probeStorage consulta o recurso no provedor e retorna o status HTTP e parte do corpo.
func probeStorage(bucket string, prov Provider, debug bool) (int, string) {
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	var target string
	switch prov {
	case ProviderAWS:
		if strings.Contains(bucket, ".") {
			target = "https://s3.amazonaws.com/" + bucket
		} else {
			target = ProviderAWS.ResourceURL(bucket)
		}
	case ProviderGCP:
		if strings.Contains(bucket, ".") {
			target = "https://storage.googleapis.com/" + bucket
		} else {
			target = ProviderGCP.ResourceURL(bucket)
		}
	case ProviderAzure:
		target = "https://" + bucket + ".blob.core.windows.net/?comp=list"
	default:
		return 0, ""
	}

	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return 0, ""
	}

	resp, err := client.Do(req)
	if err != nil {
		if debug {
			fmt.Printf("[DEBUG] takeover probe %s | erro: %v\n", target, err)
		}
		return 0, ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))

	if debug {
		fmt.Printf("[DEBUG] takeover probe %s | status %d\n", target, resp.StatusCode)
	}

	return resp.StatusCode, string(body)
}
