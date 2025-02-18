package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
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
				statusCode, body, err := getStatusCode(client, target)
				if err != nil {
					continue
				}

				title := extractTitle(body)
				if statusCode != 404 {
					printResult(target, statusCode, title)
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

func getStatusCode(client *fasthttp.Client, url string) (int, []byte, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	err := client.Do(req, resp)
	if err != nil {
		return 0, nil, err
	}
	body := make([]byte, len(resp.Body()))
	copy(body, resp.Body())
	return resp.StatusCode(), body, nil
}

func extractTitle(body []byte) string {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return "N/A"
	}
	title := strings.TrimSpace(doc.Find("title").Text())
	if title == "" {
		return "No Title"
	}
	return title
}

func printResult(url string, status int, title string) {
	var statusStr string
	switch {
	case status >= 200 && status < 300:
		statusStr = green(fmt.Sprintf("%d", status))
	case status >= 300 && status < 400:
		statusStr = blue(fmt.Sprintf("%d", status))
	case status >= 400 && status < 500:
		statusStr = yellow(fmt.Sprintf("%d", status))
	default:
		statusStr = red(fmt.Sprintf("%d", status))
	}

	// 格式化输出为表格样式
	fmt.Printf("%-40s %-10s %s\n", truncateString(url, 128), statusStr, truncateString(title, 128))
}

func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
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
