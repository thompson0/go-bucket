package buckets

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

func Brute(name string, provider Provider, stopOnFound bool, wordlistPath string, numThreads int, timeout int, outputFile string, debug bool) {
	stop := make(chan struct{})
	var path string
	var threads int

	wordlistPath = strings.TrimSpace(wordlistPath)

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
		Provider:    provider,
		Name:        ResourceName(name, provider),
	}

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go worker(&wg, jobs, ctx)
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-stop:
			close(jobs)
			wg.Wait()
			return
		case jobs <- scanner.Text():
		}
	}

	close(jobs)
	wg.Wait()

	if err := scanner.Err(); err != nil {
		fmt.Println("Erro ao ler wordlist:", err)
	}
}

type BruteContext struct {
	Timeout     time.Duration
	StopOnFound bool
	OutFile     *os.File
	WriteMu     sync.Mutex
	Stop        chan struct{}
	StopOnce    sync.Once
	Provider    Provider
	Name        string
	Debug       bool
}

func sanitizeBucketPart(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(input))
	lastSep := false

	for _, r := range input {
		isLetter := r >= 'a' && r <= 'z'
		isNumber := r >= '0' && r <= '9'
		if isLetter || isNumber {
			b.WriteRune(r)
			lastSep = false
			continue
		}
		if !lastSep {
			b.WriteByte('-')
			lastSep = true
		}
	}

	return strings.Trim(b.String(), "-.")
}

func isValidBucketName(name string) bool {
	if len(name) < 3 || len(name) > 63 {
		return false
	}

	for i, r := range name {
		isLetter := r >= 'a' && r <= 'z'
		isNumber := r >= '0' && r <= '9'
		isSeparator := r == '.' || r == '-'
		if !isLetter && !isNumber && !isSeparator {
			return false
		}
		if (i == 0 || i == len(name)-1) && !isLetter && !isNumber {
			return false
		}
	}

	if strings.Contains(name, "..") {
		return false
	}

	parts := strings.Split(name, ".")
	if len(parts) == 4 {
		looksLikeIP := true
		for _, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil || n < 0 || n > 255 {
				looksLikeIP = false
				break
			}
		}
		if looksLikeIP {
			return false
		}
	}

	return true
}

func generateVariants(name, word string) []string {
	separators := []string{"-", ".", ""}
	seen := make(map[string]struct{})
	var variants []string

	add := func(s string) {
		s = sanitizeBucketPart(s)
		if s == "" || !isValidBucketName(s) {
			return
		}
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			variants = append(variants, s)
		}
	}

	for _, sep := range separators {
		add(name + sep + word)
		add(word + sep + name)
	}

	return variants
}

func writeResult(ctx *BruteContext, line string) {
	ctx.WriteMu.Lock()
	defer ctx.WriteMu.Unlock()

	fmt.Println(line)
	if ctx.OutFile != nil {
		fmt.Fprintln(ctx.OutFile, line)
	}
}

func worker(wg *sync.WaitGroup, jobs <-chan string, ctx *BruteContext) {
	defer wg.Done()
	for word := range jobs {
		select {
		case <-ctx.Stop:
			return
		default:
			variants := generateVariants(ctx.Name, word)
			if len(variants) == 0 {
				if ctx.Debug {
					fmt.Printf("[DEBUG] ignorando entrada inválida da wordlist: %q\n", word)
				}
				continue
			}

			for _, bucket := range variants {
				select {
				case <-ctx.Stop:
					return
				default:
					targetURL := ctx.Provider.ResourceURL(bucket)
					result := CheckBucketWithTimeout(targetURL, ctx.Provider, ctx.Timeout, ctx.Debug)

					if result.Exist {
						writeResult(ctx, fmt.Sprintf("[ACHEI] %s | Region: %s", targetURL, result.Region))

						for _, m := range result.Methods {
							sym := "✗"
							if m.Allowed {
								sym = "✓"
							}
							writeResult(ctx, fmt.Sprintf("  %s %-8s %d", sym, m.Method, m.StatusCode))
						}

						if ctx.StopOnFound {
							ctx.StopOnce.Do(func() { close(ctx.Stop) })
							return
						}
					} else if result.Err != nil {
						if ctx.Debug {
							writeResult(ctx, fmt.Sprintf("[ERRO] %s | %v", targetURL, result.Err))
						}
					} else {
						writeResult(ctx, fmt.Sprintf("[NÃO ENCONTRADO] %s", targetURL))
					}
				}
			}
		}
	}
}
