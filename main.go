package main

import (
	"fmt"
	"strings"
	"go-bucket/buckets"
)

func main() {
	var url string
	fmt.Println("Digite o nome do site que deseja buscar o Bucket")
	fmt.Scan(&url)

	FormataUrl(url)
	

    result := buckets.CheckBucket(url, true)

    if result.Err != nil {
        fmt.Println("Erro:", result.Err)
        return
    }

    fmt.Println("Existe:", result.Exist)
    fmt.Println("Publico:", result.Public)
    fmt.Println("Status:", result.StatusCode)

}

func FormataUrl(input string) string {
	input = strings.TrimSpace(input)

	if strings.Contains(input, ".s3.amazonaws.com") {
		return input
	}

	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		input = "http://" + input
	}

	u, err := url.Parse(input)
	if err != nil {
		
		return fmt.Sprintf("https://%s.s3.amazonaws.com/", input)
	}

	host := u.Hostname() 

	parts := strings.Split(host, ".")

	// fallback
	return fmt.Sprintf("https://%s.s3.amazonaws.com/", host)
}
