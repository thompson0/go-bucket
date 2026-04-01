package buckets

import (
	"fmt"
	"net/http"
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

var httpMethods = []string{
	"HEAD", "GET", "PUT", "DELETE",
	"OPTIONS", "PATCH", "POST",
}

func CheckBucket(rawURL string, debug bool) BucketTest {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// HEAD primeiro para checar existência
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

	// Testa todos os métodos no bucket encontrado
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