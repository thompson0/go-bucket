package main

import (
	"fmt"
	"strings"
	"net/url"
	"go-bucket/buckets"
	"flag"
)

func main() {
	var url string
	var bruteforce string
	var stopOnFound = flag.Bool("stop-on-found", false, "Parar ao encontrar um bucket")
	var alvo = flag.String("u", "", "URL alvo para buscar")
	var wordlist = flag.String("w", "", "Caminho da wordlist")
	var threads = flag.Int("t", 1, "Número de threads")
	var timeout = flag.Int("timeout", 30, "Timeout em segundos")
	var output = flag.String("output", "", "Arquivo de saída para resultados")
	var debug = flag.Bool("debug", false, "Mostrar debug de cada requisicao")
	flag.Parse()

	if  *stopOnFound {
		fmt.Println("Modo stop ativado")
	}

	if *alvo != "" && *wordlist != "" {
		alvo := FormataUrl(*alvo)
		buckets.Brute(alvo, *stopOnFound, *wordlist, *threads, *timeout, *output, *debug)
		return
	}

	fmt.Println("Digite o nome do site que deseja buscar o Bucket")
	fmt.Scan(&url)

	url  = FormataUrl(url)
    result := buckets.CheckBucket(url, *debug)

    if result.Err != nil {
        fmt.Println("Erro:", result.Err)
        return
    }

    fmt.Println("Existe:", result.Exist)
    fmt.Println("Publico:", result.Public)
    fmt.Println("Status:", result.StatusCode)

	if result.Exist == false{
		fmt.Println("Bucket não encontrado deseja tentar um bruteforce com nomes parecidos? [S/n]")
		fmt.Scan(&bruteforce)

		if strings.ToLower(bruteforce) == "s" {
			buckets.Brute(url, *stopOnFound, *wordlist, *threads, *timeout, *output, *debug)
		}
		

	}

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

	// fallback
	return fmt.Sprintf("https://%s.s3.amazonaws.com/", u.Host)
}
