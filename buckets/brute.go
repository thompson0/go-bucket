package buckets

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"
	"strings"
)

func Brute(name string, stopOnFound bool, wordlistPath string, numThreads int, timeout int, outputFile string, debug bool) {

	stop := make(chan struct{})
	var path string
	var threads int

	wordlistPath = strings.TrimSpace(wordlistPath)

	// Se wordlistPath não foi fornecido, pedir ao usuário
	if wordlistPath == "" {
		fmt.Print("Digite o caminho da wordlist: ")
		fmt.Scan(&path)

		fmt.Print("Digite a quantidade de threads: ")
		fmt.Scan(&threads)
	} else {
		path = wordlistPath
		threads = numThreads
		if threads <= 0 {
			threads = 1
		}
	}

	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir wordlist:", err)
		return
	}
	defer file.Close()

	jobs := make(chan string, 100)
	var wg sync.WaitGroup
	var outFile *os.File
	var err2 error

	if outputFile != "" {
		outFile, err2 = os.Create(outputFile)
		if err2 != nil {
			fmt.Println("Erro ao criar arquivo de saída:", err2)
			return
		}
		defer outFile.Close()
	}

	ctx := &BruteContext{
		Timeout:     time.Duration(timeout) * time.Second,
		StopOnFound: stopOnFound,
		OutFile:     outFile,
		Stop:        stop,
		Debug:       debug,
		Name: 		 normalizeBucketName(name),
	}

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go worker(&wg, jobs, ctx)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		jobs <- scanner.Text()
	}

	close(jobs)
	wg.Wait()
}

type BruteContext struct {
	Timeout     time.Duration
	StopOnFound bool
	OutFile     *os.File
	Stop        chan struct{}
	Name        string
	Debug       bool
}

func normalizeBucketName(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return input
	}

	if u, err := url.Parse(input); err == nil && u.Host != "" {
		input = u.Host
	}

	input = strings.TrimPrefix(input, "http://")
	input = strings.TrimPrefix(input, "https://")
	input = strings.TrimSuffix(input, "/")

	if idx := strings.Index(input, "/"); idx != -1 {
		input = input[:idx]
	}

	input = strings.TrimSuffix(input, ".s3.amazonaws.com")
	return input
}



func worker(wg *sync.WaitGroup, jobs <-chan string, ctx *BruteContext) {
	defer wg.Done()

	for word := range jobs {
		select {
		case <-ctx.Stop:
			return
		default:
			bucket := fmt.Sprintf("%s-%s", ctx.Name, word)
			url := fmt.Sprintf("https://%s.s3.amazonaws.com/", bucket)

			result := CheckBucket(url, ctx.Debug)
			
			if result.Exist {
				msg := fmt.Sprintf("[ACHEI] %s", url)
				fmt.Println(msg)
				if ctx.OutFile != nil {
					fmt.Fprintln(ctx.OutFile, msg)
				}
				if ctx.StopOnFound {
					close(ctx.Stop)
					return
				}
			}
		}
	}
}