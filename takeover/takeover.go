package takeover

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go-bucket/buckets"
	"io"
	"net/http"
	"strings"
	"time"
)

// Credentials contém as credenciais para o PoC de criação de bucket.
type Credentials struct {
	AccessKey string
	SecretKey string
	Region    string
}

// CriarBucket é o PoC que reivindica um bucket no provedor.
// AWS usa assinatura SigV4 manual; Azure/GCP imprimem o comando pronto
// (criação via SDK exigiria OAuth/ARM, fora do escopo de uma tool stdlib-only).
func CriarBucket(provider buckets.Provider, bucket string, creds Credentials, debug bool) error {
	if !buckets.ValidBucketName(bucket) {
		return fmt.Errorf("nome de bucket inválido: %s", bucket)
	}

	switch provider {
	case buckets.ProviderAWS:
		return awsCreateBucket(bucket, creds, debug)
	case buckets.ProviderAzure:
		printAzureInstructions(bucket)
		return nil
	case buckets.ProviderGCP:
		fmt.Printf("gcloud storage buckets create gs://%s\n", bucket)
		return nil
	default:
		return fmt.Errorf("provedor não suportado: %s", provider)
	}
}

// awsCreateBucket cria o bucket via API CreateBucket com assinatura SigV4 manual.
func awsCreateBucket(bucket string, creds Credentials, debug bool) error {
	if creds.AccessKey == "" || creds.SecretKey == "" {
		return fmt.Errorf("credenciais AWS necessárias (access key e secret key)")
	}

	region := creds.Region
	if region == "" {
		region = "us-east-1"
	}

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := amzDate[:8]

	// Corpo: vazio em us-east-1; LocationConstraint obrigatório fora dela.
	var body []byte
	var payloadHash string
	if region == "us-east-1" {
		payloadHash = sha256Hex(nil)
	} else {
		body = []byte(fmt.Sprintf(
			`<CreateBucketConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><LocationConstraint>%s</LocationConstraint></CreateBucketConfiguration>`,
			region,
		))
		payloadHash = sha256Hex(body)
	}

	host := "s3.amazonaws.com"
	if region != "us-east-1" {
		host = "s3." + region + ".amazonaws.com"
	}

	canonicalURI := "/" + awsURIEncode(bucket)
	canonicalQuery := ""
	canonicalHeaders := strings.Join([]string{
		"host:" + host,
		"x-amz-acl:private",
		"x-amz-content-sha256:" + payloadHash,
		"x-amz-date:" + amzDate,
	}, "\n") + "\n"
	signedHeaders := "host;x-amz-acl;x-amz-content-sha256;x-amz-date"

	canonicalRequest := strings.Join([]string{
		"PUT",
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := dateStamp + "/" + region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := hmacSHA256([]byte("AWS4"+creds.SecretKey), dateStamp)
	signingKey = hmacSHA256(signingKey, region)
	signingKey = hmacSHA256(signingKey, "s3")
	signingKey = hmacSHA256(signingKey, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	authHeader := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		creds.AccessKey, scope, signedHeaders, signature,
	)

	target := "https://" + host + canonicalURI
	req, err := http.NewRequest("PUT", target, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("erro ao criar request: %v", err)
	}
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("x-amz-acl", "private")
	req.Header.Set("Authorization", authHeader)

	if debug {
		fmt.Printf("[DEBUG] PUT %s | região %s | body len %d\n", target, region, len(body))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("erro ao enviar request: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))

	if debug {
		fmt.Printf("[DEBUG] PUT %s -> HTTP %d\n", target, resp.StatusCode)
	}

	switch resp.StatusCode {
	case 200:
		fmt.Printf("[+] Bucket %s criado com sucesso no S3 (região %s)\n", bucket, region)
		return nil
	case 409:
		return fmt.Errorf("bucket já existe (HTTP 409 BucketAlreadyExists)")
	case 403:
		return fmt.Errorf("credenciais inválidas ou sem permissão (HTTP 403)")
	case 400:
		if strings.Contains(strings.ToLower(string(respBody)), "signature") {
			return fmt.Errorf("erro de assinatura SigV4 (HTTP 400 SignatureDoesNotMatch)")
		}
		return fmt.Errorf("request malformado (HTTP 400): %s", strings.TrimSpace(string(respBody)))
	case 301:
		return fmt.Errorf("região errada (HTTP 301) — o bucket pode existir em outra região")
	default:
		return fmt.Errorf("resposta inesperada (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
}

// printAzureInstructions imprime os comandos az prontos para reivindicar o storage account.
func printAzureInstructions(bucket string) {
	fmt.Println("Criar um storage account do Azure exige autenticação OAuth/ARM,")
	fmt.Println("fora do escopo desta tool (sem SDK). Para reivindicar manualmente:")
	fmt.Printf("  az storage account create -n %s -g <resource-group> -l <location> --sku Standard_LRS\n", bucket)
	fmt.Printf("  az storage container create --account-name %s -n %s\n", bucket, bucket)
}

// sha256Hex retorna o hash SHA-256 em hex minúsculo.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// hmacSHA256 aplica HMAC-SHA256 com a chave informada.
func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// awsURIEncode codifica a URI conforme as regras do SigV4
// (tudo é percent-encoded exceto A-Za-z0-9-._~).
func awsURIEncode(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '.', r == '_', r == '~':
			b.WriteRune(r)
		default:
			for _, c := range []byte(string(r)) {
				fmt.Fprintf(&b, "%%%02X", c)
			}
		}
	}
	return b.String()
}
