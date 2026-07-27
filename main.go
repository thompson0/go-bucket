package main

import (
	"bufio"
	"flag"
	"fmt"
	"go-bucket/buckets"
	"go-bucket/db"
	"go-bucket/draw"
	"os"
	"strings"
)

func main() {

	draw.BucketDraw()

	flag.Usage = func() {
		fmt.Println("Bucket Scanner")
		fmt.Println("")
		fmt.Println("Uso:")
		fmt.Println("  go run main.go -u <alvo> -w <wordlist> -provider <aws|azure> [opções]")
		fmt.Println("")
		flag.PrintDefaults()
	}
	var bruteforce string
	var stopOnFound = flag.Bool("stop-on-found", false, "Parar ao encontrar um bucket")
	var alvo = flag.String("u", "", "URL alvo para buscar")
	var wordlist = flag.String("w", "", "Caminho da wordlist")
	var threads = flag.Int("t", 1, "Número de threads")
	var timeout = flag.Int("timeout", 30, "Timeout em segundos")
	var output = flag.String("output", "", "Arquivo de saída para resultados")
	var debug = flag.Bool("debug", false, "Mostrar debug de cada requisicao")
	var dns = flag.String("dns", "", "Domínio para resolver e detectar se é um bucket/storage")
	var providerFlag = flag.String("provider", "aws", "Provedor alvo: aws ou azure")
	flag.Parse()

	provider, err := buckets.ParseProvider(*providerFlag)
	if err != nil {
		fmt.Println("Erro:", err)
		return
	}

	store, err := db.Init()
	if err != nil {
		fmt.Println("Erro ao iniciar store em memoria:", err)
		return
	}

	if *stopOnFound {
		fmt.Println("Modo stop ativado")
	}

	if *dns != "" {
		fmt.Printf("[*] Resolvendo %s...\n\n", *dns)
		dnsResult := buckets.DnsResolver(*dns, *debug)
		printDNSResolverResult(dnsResult)
		return
	}

	if *alvo != "" && *wordlist != "" {
		alvo := buckets.FormatBucketURL(*alvo, provider)

		if !*debug {
			totalLines := countWordlistLines(*wordlist)
			if totalLines > 0 {
				fmt.Printf("[*] Wordlist carregada com %d entradas\n", totalLines)
			}
		}

		buckets.Brute(alvo, provider, *stopOnFound, *wordlist, *threads, *timeout, *output, *debug)
		return
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("")
	fmt.Println("Deseja resolver um domínio para detectar se é um bucket/storage? [S/n]")
	dnsResp, _ := reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(dnsResp)) == "s" || strings.TrimSpace(dnsResp) == "" {
		fmt.Print("Digite o domínio para resolver: ")
		dominio, _ := reader.ReadString('\n')
		dominio = strings.TrimSpace(dominio)
		if dominio != "" {
			fmt.Printf("\n[*] Resolvendo %s...\n\n", dominio)
			dnsResult := buckets.DnsResolver(dominio, *debug)
			printDNSResolverResult(dnsResult)
		}
	}

	fmt.Println("")
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

		url := buckets.NormalizeBucketURL(input, provider)

		if cached, ok := db.Get(store, url); ok {
			fmt.Println("Resultado recuperado da memoria")
			fmt.Println("Existe:", cached.Exist)
			fmt.Println("Publico:", cached.Public)
			fmt.Println("Status:", cached.StatusCode)
			continue
		}

		result := buckets.CheckBucket(url, provider, *debug)
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
				buckets.Brute(url, provider, *stopOnFound, *wordlist, *threads, *timeout, *output, *debug)
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

func printDNSResolverResult(result buckets.DNSResolverResult) {
	fmt.Println("=== RESULTADO DA RESOLUÇÃO DNS ===")

	if result.IsReachable {
		fmt.Printf("✓ Acessível como storage\n")
		fmt.Printf("  Provider: %s\n", result.Provider)
		fmt.Printf("  Método: %s\n", result.Method)
		fmt.Printf("  Detalhes: %s\n", result.Details)
	} else {
		fmt.Printf("✗ Não identificado como storage\n")
		if result.Error != "" {
			fmt.Printf("  Motivo: %s\n", result.Error)
		}
	}

	fmt.Println("===================================")
	fmt.Println()
}

func countWordlistLines(wordlistPath string) int {
	file, err := os.Open(wordlistPath)
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count
}
