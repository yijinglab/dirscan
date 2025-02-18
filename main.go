package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/valyala/fasthttp"
)

var (
	url      = flag.String("u", "", "Target URL")
	urlFile  = flag.String("U", "", "URL list file")
	wordlist = flag.String("w", "", "Directory wordlist file")
	threads  = flag.Int("t", 10, "Number of threads")
	help     = flag.Bool("h", false, "Show help information")
)

var (
	red    = color.New(color.FgRed).SprintFunc()
	green  = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	blue   = color.New(color.FgBlue).SprintFunc()
)

func main() {
	flag.Parse()

	if *help || (*url == "" && *urlFile == "") || *wordlist == "" {
		printHelp()
		return
	}

	urls := getURLs()
	dirs := getDirectories()

	var wg sync.WaitGroup
	jobs := make(chan string, *threads*2)

	// Start workers
	for i := 0; i < *threads; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Worker panic: %v\n", r)
				}
			}()
			worker(jobs, &wg, urls)
		}()
	}

	// Add all jobs first
	wg.Add(len(dirs))

	// Send jobs
	go func() {
		for _, dir := range dirs {
			jobs <- dir
		}
	}()

	wg.Wait()
	close(jobs)
}

func worker(jobs <-chan string, wg *sync.WaitGroup, urls []string) {
	client := &fasthttp.Client{
		Name: "DirScan",
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Worker recovered from panic: %v\n", r)
		}
	}()

	for dir := range jobs {
		func() {
			defer wg.Done()
			for _, baseURL := range urls {
				target := formatURL(baseURL, dir)
				statusCode, err := getStatusCode(client, target)
				if err != nil {
					continue
				}

				if statusCode != 404 {
					printResult(target, statusCode)
				}
			}
		}()
	}
}

func formatURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	path = strings.TrimLeft(path, "/")
	return base + "/" + path
}

func getStatusCode(client *fasthttp.Client, url string) (int, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	err := client.Do(req, resp)
	if err != nil {
		return 0, err
	}
	return resp.StatusCode(), nil
}

func printResult(url string, status int) {
	var statusStr string
	switch {
	case status >= 200 && status < 300:
		statusStr = green(status)
	case status >= 300 && status < 400:
		statusStr = blue(status)
	case status >= 400 && status < 500:
		statusStr = yellow(status)
	default:
		statusStr = red(status)
	}
	fmt.Printf("[%s] %s\n", statusStr, url)
}

func getURLs() []string {
	var urls []string

	if *url != "" {
		urls = append(urls, *url)
	}

	if *urlFile != "" {
		file, err := os.Open(*urlFile)
		if err != nil {
			fmt.Println(red("Error opening URL file:"), err)
			os.Exit(1)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			url := strings.TrimSpace(scanner.Text())
			if url != "" {
				urls = append(urls, url)
			}
		}
	}

	return urls
}

func getDirectories() []string {
	var dirs []string

	file, err := os.Open(*wordlist)
	if err != nil {
		fmt.Println(red("Error opening wordlist file:"), err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		dir := strings.TrimSpace(scanner.Text())
		if dir != "" {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func printHelp() {
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println("Directory Scanner - Fast HTTP directory brute-forcer")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println("Usage:")
	flag.PrintDefaults()
	fmt.Println("\nExamples:")
	fmt.Println("  Scan single URL: dirscan -u http://example.com -w paths.txt")
	fmt.Println("  Scan URL list: dirscan -U urls.txt -w paths.txt -t 20")
	fmt.Println(strings.Repeat("-", 50))
}
