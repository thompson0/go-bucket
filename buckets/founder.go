package buckets

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

type MethodResult struct {
	Method     string
	StatusCode int
	Allowed    bool
}

type BucketTest struct {
	Exist      bool
	Public     bool
	StatusCode int
	Err        error
	Region     string
	Methods    []MethodResult
}

// DNSResolverResult contém informações sobre a resolução DNS
type DNSResolverResult struct {
	IsReachable bool
	Provider    string
	Method      string
	Details     string
	Error       string
}

var httpMethods = []string{
	"HEAD", "GET", "PUT", "DELETE",
	"OPTIONS", "PATCH", "POST",
}

func CheckBucket(rawURL string, provider Provider, debug bool) BucketTest {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	headReq, err := http.NewRequest("HEAD", rawURL, nil)
	if err != nil {
		return BucketTest{Err: err}
	}

	headResp, err := client.Do(headReq)
	if err != nil {
		if debug {
			fmt.Printf("[DEBUG] %s | HEAD error: %v\n", rawURL, err)
		}
		return BucketTest{Err: err}
	}
	defer headResp.Body.Close()

	result := BucketTest{
		StatusCode: headResp.StatusCode,
	}

	region := headResp.Header.Get("x-amz-bucket-region")
	if region != "" {
		result.Region = region
	}

	switch headResp.StatusCode {
	case 200:
		result.Exist = true
		result.Public = true
	case 403:
		result.Exist = true
		result.Public = false
	case 301:
		result.Exist = true
	case 404:
		result.Exist = false
	}

	if !result.Exist {
		if debug {
			fmt.Printf("[DEBUG] %s | bucket não existe (status %d)\n", rawURL, headResp.StatusCode)
		}
		return result
	}

	for _, method := range httpMethods {
		req, err := http.NewRequest(method, rawURL, nil)
		if err != nil {
			if debug {
				fmt.Printf("[DEBUG] %s | %s | erro ao criar request: %v\n", rawURL, method, err)
			}
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			if debug {
				fmt.Printf("[DEBUG] %s | %s | erro: %v\n", rawURL, method, err)
			}
			continue
		}
		resp.Body.Close()

		mr := MethodResult{
			Method:     method,
			StatusCode: resp.StatusCode,
			Allowed:    resp.StatusCode != 403 && resp.StatusCode != 405,
		}

		result.Methods = append(result.Methods, mr)

		if debug {
			status := "NEGADO"
			if mr.Allowed {
				status = "PERMITIDO"
			}
			fmt.Printf("[DEBUG] %s | %s | %d | %s\n", rawURL, method, resp.StatusCode, status)
		}
	}

	return result
}

// DnsResolver resolve um domínio e detecta o provedor de storage
func DnsResolver(dominio string, debug bool) DNSResolverResult {
	result := DNSResolverResult{
		IsReachable: false,
		Provider:    "",
		Method:      "",
		Details:     "",
		Error:       "",
	}

	if strings.TrimSpace(dominio) == "" {
		result.Error = "domínio vazio"
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Tenta resolver CNAME
	cname, err := net.DefaultResolver.LookupCNAME(ctx, dominio)
	if err == nil && cname != "" {
		cname = strings.ToLower(cname)

		if strings.Contains(cname, "amazonaws.com") {
			result.IsReachable = true
			result.Provider = "aws"
			result.Method = "DNS CNAME"
			result.Details = fmt.Sprintf("AWS S3 CNAME: %s", cname)
			return result
		}
		if strings.Contains(cname, "googleapis.com") || strings.Contains(cname, "commondatastorage") {
			result.IsReachable = true
			result.Provider = "gcp"
			result.Method = "DNS CNAME"
			result.Details = fmt.Sprintf("Google Cloud Storage CNAME: %s", cname)
			return result
		}
		if strings.Contains(cname, "blob.core.windows.net") || strings.Contains(cname, "azureedge.net") {
			result.IsReachable = true
			result.Provider = "azure"
			result.Method = "DNS CNAME"
			result.Details = fmt.Sprintf("Azure Blob CNAME: %s", cname)
			return result
		}
	}

	// Verifica via HTTP headers
	httpResult := checkViaHTTP(ctx, dominio, debug)
	if httpResult.IsReachable {
		return httpResult
	}

	result.Error = "Não foi possível resolver o domínio como bucket de storage"
	return result
}

// checkViaHTTP verifica o provedor através de headers HTTP
func checkViaHTTP(ctx context.Context, dominio string, debug bool) DNSResolverResult {
	result := DNSResolverResult{
		IsReachable: false,
		Provider:    "",
		Method:      "HTTP Headers",
		Details:     "",
		Error:       "",
	}

	client := &http.Client{
		Timeout: 3 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Tenta HTTPS primeiro
	targetURL := "https://" + dominio
	req, err := http.NewRequestWithContext(ctx, "HEAD", targetURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("erro ao criar request: %v", err)
		return result
	}

	resp, err := client.Do(req)
	if err != nil {
		// Tenta HTTP como fallback
		targetURL = "http://" + dominio
		req, err = http.NewRequestWithContext(ctx, "HEAD", targetURL, nil)
		if err != nil {
			result.Error = fmt.Sprintf("domínio inacessível: %v", err)
			return result
		}
		resp, err = client.Do(req)
		if err != nil {
			result.Error = fmt.Sprintf("domínio inacessível via HTTP e HTTPS: %v", err)
			return result
		}
	}
	defer resp.Body.Close()

	// Analisa headers
	serverHeader := strings.ToLower(resp.Header.Get("Server"))
	xAmzId2 := resp.Header.Get("x-amz-id-2")
	xAmzBucketRegion := resp.Header.Get("x-amz-bucket-region")
	xGoogMetageneration := resp.Header.Get("x-goog-metageneration")
	xMsBlobType := resp.Header.Get("x-ms-blob-type")

	if debug {
		fmt.Printf("[DEBUG] DNS: %s | Status: %d | Server: %s\n", dominio, resp.StatusCode, serverHeader)
	}

	// AWS S3
	if serverHeader == "amazon s3" || xAmzId2 != "" || xAmzBucketRegion != "" {
		result.IsReachable = true
		result.Provider = "aws"
		details := "Amazon S3"
		if xAmzBucketRegion != "" {
			details = fmt.Sprintf("Amazon S3 (Region: %s)", xAmzBucketRegion)
		}
		result.Details = details
		return result
	}

	// Google Cloud Storage
	if strings.Contains(serverHeader, "gcs") || xGoogMetageneration != "" || resp.Header.Get("x-goog-storage-class") != "" {
		result.IsReachable = true
		result.Provider = "gcp"
		result.Details = "Google Cloud Storage"
		return result
	}

	// Azure Blob Storage
	if serverHeader == "windows-azure-blob" || xMsBlobType != "" || resp.Header.Get("x-ms-version") != "" {
		result.IsReachable = true
		result.Provider = "azure"
		result.Details = "Azure Blob Storage"
		return result
	}

	// Status code 403 é comum em buckets com acesso restrito
	if resp.StatusCode == 403 {
		result.IsReachable = true
		result.Details = fmt.Sprintf("Storage acessível (HTTP %d - acesso restrito)", resp.StatusCode)
		return result
	}

	// Não identificou como storage
	result.Error = fmt.Sprintf("Não é um bucket visível ou está atrás de Proxy/CDN (HTTP %d)", resp.StatusCode)
	return result
}
