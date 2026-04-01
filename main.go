package main

import (
	"bufio"
	"fmt"
	"strings"
	"net/url"
	"go-bucket/buckets"
	"go-bucket/db"
	"flag"
	"os"
)

func main() {
	var bruteforce string
	var stopOnFound = flag.Bool("stop-on-found", false, "Parar ao encontrar um bucket")
	var alvo = flag.String("u", "", "URL alvo para buscar")
	var wordlist = flag.String("w", "", "Caminho da wordlist")
	var threads = flag.Int("t", 1, "Número de threads")
	var timeout = flag.Int("timeout", 30, "Timeout em segundos")
	var output = flag.String("output", "", "Arquivo de saída para resultados")
	var debug = flag.Bool("debug", false, "Mostrar debug de cada requisicao")
	flag.Parse()

	store, err := db.Init()
	if err != nil {
		fmt.Println("Erro ao iniciar store em memoria:", err)
		return
	}

	if  *stopOnFound {
		fmt.Println("Modo stop ativado")
	}

	if *alvo != "" && *wordlist != "" {
		alvo := FormataUrl(*alvo)
		buckets.Brute(alvo, *stopOnFound, *wordlist, *threads, *timeout, *output, *debug)
		return
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("Digite o nome do site que deseja buscar o Bucket")
		input, readErr := reader.ReadString('\n')
		if readErr != nil {
			fmt.Println("Erro ao ler entrada:", readErr)
			return
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		url := NormalizeBucketURL(input)

		if cached, ok := db.Get(store, url); ok {
			fmt.Println("Resultado recuperado da memoria")
			fmt.Println("Existe:", cached.Exist)
			fmt.Println("Publico:", cached.Public)
			fmt.Println("Status:", cached.StatusCode)
			continue
		}

		result := buckets.CheckBucket(url, *debug)
		if result.Err != nil {
			fmt.Println("Erro:", result.Err)
			continue
		}

		db.Save(store, url, db.BucketTest{
			Exist:      result.Exist,
			Public:     result.Public,
			StatusCode: result.StatusCode,
			Region:     result.Region,
		})
		fmt.Println("Resultado salvo em memoria")

		fmt.Println("Existe:", result.Exist)
		fmt.Println("Publico:", result.Public)
		fmt.Println("Status:", result.StatusCode)
		printAllowedMethods(result.Methods)

		if !result.Exist {
			fmt.Println("Bucket não encontrado deseja tentar um bruteforce com nomes parecidos? [S/n]")
			resp, readErr := reader.ReadString('\n')
			if readErr != nil {
				fmt.Println("Erro ao ler entrada:", readErr)
				continue
			}

			bruteforce = strings.TrimSpace(strings.ToLower(resp))
			if bruteforce == "" || bruteforce == "s" {
				buckets.Brute(url, *stopOnFound, *wordlist, *threads, *timeout, *output, *debug)
			}
		}
	}

}

func printAllowedMethods(methods []buckets.MethodResult) {
	if len(methods) == 0 {
		fmt.Println("Metodos permitidos: nenhum identificado")
		return
	}

	var allowed []string
	for _, m := range methods {
		if m.Allowed {
			allowed = append(allowed, fmt.Sprintf("%s(%d)", m.Method, m.StatusCode))
		}
	}

	if len(allowed) == 0 {
		fmt.Println("Metodos permitidos: nenhum")
		return
	}

	fmt.Printf("Metodos permitidos: %s\n", strings.Join(allowed, ", "))
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

func NormalizeBucketURL(input string) string {
	formatted := FormataUrl(input)
	formatted = strings.TrimSpace(formatted)

	u, err := url.Parse(formatted)
	if err != nil || u.Host == "" {
		return formatted
	}

	host := strings.ToLower(strings.TrimSpace(u.Host))
	host = strings.TrimSuffix(host, "/")

	return fmt.Sprintf("https://%s/", host)
}
