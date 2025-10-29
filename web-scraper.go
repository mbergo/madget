package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type Scraper struct {
	baseURL        *url.URL
	maxDepth       int
	maxWorkers     int
	visited        map[string]bool
	visitedMu      sync.RWMutex
	queue          chan Job
	wg             sync.WaitGroup
	client         *http.Client
	outputDir      string
	httpTracking   bool
	convertLinks   bool
	downloadStats  Stats
	statsMu        sync.Mutex
	// Track URL to local file path mapping for link conversion
	urlToPath      map[string]string
	urlToPathMu    sync.RWMutex
	htmlFiles      []string
	htmlFilesMu    sync.Mutex
}

type Job struct {
	url   string
	depth int
}

type Stats struct {
	TotalRequests   int
	SuccessRequests int
	FailedRequests  int
	BytesDownloaded int64
	StartTime       time.Time
}

func NewScraper(baseURL string, maxDepth, maxWorkers int, outputDir string, httpTracking, convertLinks bool) (*Scraper, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Create HTTP client with custom transport
	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}

	return &Scraper{
		baseURL:      parsedURL,
		maxDepth:     maxDepth,
		maxWorkers:   maxWorkers,
		visited:      make(map[string]bool),
		queue:        make(chan Job, maxWorkers*10),
		client:       client,
		outputDir:    outputDir,
		httpTracking: httpTracking,
		convertLinks: convertLinks,
		downloadStats: Stats{
			StartTime: time.Now(),
		},
		urlToPath: make(map[string]string),
		htmlFiles: make([]string, 0),
	}, nil
}

func (s *Scraper) isVisited(urlStr string) bool {
	s.visitedMu.RLock()
	defer s.visitedMu.RUnlock()
	return s.visited[urlStr]
}

func (s *Scraper) markVisited(urlStr string) {
	s.visitedMu.Lock()
	defer s.visitedMu.Unlock()
	s.visited[urlStr] = true
}

func (s *Scraper) isSameDomain(targetURL *url.URL) bool {
	return targetURL.Host == s.baseURL.Host || targetURL.Host == ""
}

func (s *Scraper) downloadPage(ctx context.Context, urlStr string, depth int) error {
	if s.isVisited(urlStr) {
		return nil
	}

	s.markVisited(urlStr)

	// Update stats
	s.statsMu.Lock()
	s.downloadStats.TotalRequests++
	s.statsMu.Unlock()

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set Chrome user agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	if s.httpTracking {
		fmt.Printf("[HTTP] GET %s (depth: %d)\n", urlStr, depth)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.statsMu.Lock()
		s.downloadStats.FailedRequests++
		s.statsMu.Unlock()
		return fmt.Errorf("failed to fetch %s: %w", urlStr, err)
	}
	defer resp.Body.Close()

	if s.httpTracking {
		fmt.Printf("[HTTP] %d %s (Content-Length: %d)\n", resp.StatusCode, urlStr, resp.ContentLength)
	}

	if resp.StatusCode != http.StatusOK {
		s.statsMu.Lock()
		s.downloadStats.FailedRequests++
		s.statsMu.Unlock()
		return fmt.Errorf("bad status code: %d for %s", resp.StatusCode, urlStr)
	}

	// Parse URL for file path
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return err
	}

	// Determine file path
	filePath := s.getFilePath(parsedURL, resp.Header.Get("Content-Type"))

	// Create directory structure
	dir := filepath.Join(s.outputDir, filepath.Dir(filePath))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Save file
	fullPath := filepath.Join(s.outputDir, filePath)
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy response body to file and track bytes
	bytesWritten, err := io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	s.statsMu.Lock()
	s.downloadStats.BytesDownloaded += bytesWritten
	s.downloadStats.SuccessRequests++
	s.statsMu.Unlock()

	// Track URL to file path mapping for link conversion
	if s.convertLinks {
		s.urlToPathMu.Lock()
		s.urlToPath[urlStr] = filePath
		s.urlToPathMu.Unlock()

		// Track HTML files for post-processing
		if strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
			s.htmlFilesMu.Lock()
			s.htmlFiles = append(s.htmlFiles, fullPath)
			s.htmlFilesMu.Unlock()
		}
	}

	fmt.Printf("✓ Saved: %s (%d bytes)\n", filePath, bytesWritten)

	// Parse HTML and extract links if we haven't reached max depth
	if depth < s.maxDepth && strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		// Re-read the file for parsing since we already consumed the body
		file.Seek(0, 0)
		links, err := s.extractLinks(file, parsedURL)
		if err != nil {
			return fmt.Errorf("failed to extract links: %w", err)
		}

		// Queue new jobs
		for _, link := range links {
			if !s.isVisited(link) && s.shouldFollow(link) {
				s.wg.Add(1)
				select {
				case s.queue <- Job{url: link, depth: depth + 1}:
				case <-ctx.Done():
					s.wg.Done()
					return ctx.Err()
				}
			}
		}
	}

	return nil
}

func (s *Scraper) shouldFollow(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Only follow same domain
	if !s.isSameDomain(parsedURL) {
		return false
	}

	// Skip common non-HTML resources
	ext := strings.ToLower(path.Ext(parsedURL.Path))
	skipExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".pdf": true, ".zip": true, ".tar": true, ".gz": true,
		".mp4": true, ".avi": true, ".mov": true, ".mp3": true,
	}

	return !skipExts[ext]
}

func (s *Scraper) getFilePath(parsedURL *url.URL, contentType string) string {
	urlPath := parsedURL.Path
	if urlPath == "" || urlPath == "/" {
		urlPath = "/index.html"
	}

	// If path doesn't have extension and is HTML, add .html
	if !strings.Contains(path.Base(urlPath), ".") && strings.Contains(contentType, "text/html") {
		urlPath = path.Join(urlPath, "index.html")
	}

	// Clean the path
	urlPath = strings.TrimPrefix(urlPath, "/")
	if urlPath == "" {
		urlPath = "index.html"
	}

	// Add host as subdirectory
	return filepath.Join(parsedURL.Host, urlPath)
}

func (s *Scraper) extractLinks(r io.Reader, baseURL *url.URL) ([]string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	var links []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var link string
			switch n.Data {
			case "a", "link":
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						link = attr.Val
					}
				}
			case "script", "img":
				for _, attr := range n.Attr {
					if attr.Key == "src" {
						link = attr.Val
					}
				}
			}

			if link != "" {
				absURL := s.makeAbsoluteURL(link, baseURL)
				if absURL != "" {
					links = append(links, absURL)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return links, nil
}

func (s *Scraper) makeAbsoluteURL(link string, base *url.URL) string {
	// Skip mailto, javascript, etc.
	if strings.HasPrefix(link, "mailto:") || strings.HasPrefix(link, "javascript:") || strings.HasPrefix(link, "#") {
		return ""
	}

	parsedURL, err := url.Parse(link)
	if err != nil {
		return ""
	}

	// Make absolute URL
	absURL := base.ResolveReference(parsedURL)

	// Remove fragment
	absURL.Fragment = ""

	return absURL.String()
}

func (s *Scraper) convertLinksInHTML(htmlPath string) error {
	// Read the HTML file
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		return fmt.Errorf("failed to read HTML file: %w", err)
	}

	htmlStr := string(content)
	modified := false

	// Get the directory of the current HTML file for calculating relative paths
	htmlDir := filepath.Dir(htmlPath)

	// Convert each known URL to a relative path
	s.urlToPathMu.RLock()
	for absURL, targetPath := range s.urlToPath {
		// Calculate relative path from current HTML file to target
		targetFullPath := filepath.Join(s.outputDir, targetPath)
		relPath, err := filepath.Rel(htmlDir, targetFullPath)
		if err != nil {
			continue
		}

		// Convert backslashes to forward slashes for URLs (Windows compatibility)
		relPath = filepath.ToSlash(relPath)

		// Replace all occurrences of the absolute URL with the relative path
		// We need to be careful to match URLs in different contexts
		replacements := []struct {
			pattern string
			replace string
		}{
			// href="absolute_url"
			{fmt.Sprintf(`href="%s"`, absURL), fmt.Sprintf(`href="%s"`, relPath)},
			// href='absolute_url'
			{fmt.Sprintf(`href='%s'`, absURL), fmt.Sprintf(`href='%s'`, relPath)},
			// src="absolute_url"
			{fmt.Sprintf(`src="%s"`, absURL), fmt.Sprintf(`src="%s"`, relPath)},
			// src='absolute_url'
			{fmt.Sprintf(`src='%s'`, absURL), fmt.Sprintf(`src='%s'`, relPath)},
		}

		for _, r := range replacements {
			if strings.Contains(htmlStr, r.pattern) {
				htmlStr = strings.ReplaceAll(htmlStr, r.pattern, r.replace)
				modified = true
			}
		}
	}
	s.urlToPathMu.RUnlock()

	// Only write back if we made changes
	if modified {
		if err := os.WriteFile(htmlPath, []byte(htmlStr), 0644); err != nil {
			return fmt.Errorf("failed to write modified HTML: %w", err)
		}
	}

	return nil
}

func (s *Scraper) convertAllLinks() error {
	if !s.convertLinks {
		return nil
	}

	fmt.Printf("\n🔗 Converting absolute links to relative...\n")

	s.htmlFilesMu.Lock()
	htmlFiles := make([]string, len(s.htmlFiles))
	copy(htmlFiles, s.htmlFiles)
	s.htmlFilesMu.Unlock()

	converted := 0
	for _, htmlFile := range htmlFiles {
		if err := s.convertLinksInHTML(htmlFile); err != nil {
			fmt.Printf("✗ Failed to convert links in %s: %v\n", htmlFile, err)
		} else {
			converted++
		}
	}

	fmt.Printf("✓ Converted links in %d HTML files\n", converted)

	return nil
}

func (s *Scraper) worker(ctx context.Context, id int) {
	defer s.wg.Done()

	for {
		select {
		case job, ok := <-s.queue:
			if !ok {
				return
			}

			if err := s.downloadPage(ctx, job.url, job.depth); err != nil {
				fmt.Printf("✗ Worker %d error: %v\n", id, err)
			}
			s.wg.Done()

		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) Run(ctx context.Context) error {
	// Create output directory
	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("🚀 Starting scraper...\n")
	fmt.Printf("   Base URL: %s\n", s.baseURL.String())
	fmt.Printf("   Max Depth: %d\n", s.maxDepth)
	fmt.Printf("   Workers: %d\n", s.maxWorkers)
	fmt.Printf("   Output: %s\n", s.outputDir)
	fmt.Printf("   HTTP Tracking: %v\n\n", s.httpTracking)

	// Start workers
	for i := 0; i < s.maxWorkers; i++ {
		s.wg.Add(1)
		go s.worker(ctx, i)
	}

	// Add initial job
	s.wg.Add(1)
	s.queue <- Job{url: s.baseURL.String(), depth: 0}

	// Wait for all jobs to complete
	s.wg.Wait()
	close(s.queue)

	// Convert absolute links to relative if requested
	if err := s.convertAllLinks(); err != nil {
		return fmt.Errorf("failed to convert links: %w", err)
	}

	// Print statistics
	s.printStats()

	return nil
}

func (s *Scraper) printStats() {
	duration := time.Since(s.downloadStats.StartTime)
	fmt.Printf("\n📊 Download Statistics:\n")
	fmt.Printf("   Duration: %s\n", duration.Round(time.Millisecond))
	fmt.Printf("   Total Requests: %d\n", s.downloadStats.TotalRequests)
	fmt.Printf("   Successful: %d\n", s.downloadStats.SuccessRequests)
	fmt.Printf("   Failed: %d\n", s.downloadStats.FailedRequests)
	fmt.Printf("   Bytes Downloaded: %s\n", formatBytes(s.downloadStats.BytesDownloaded))
	if duration.Seconds() > 0 {
		fmt.Printf("   Average Speed: %s/s\n", formatBytes(int64(float64(s.downloadStats.BytesDownloaded)/duration.Seconds())))
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func main() {
	// Command line flags
	maxDepth := flag.Int("depth", 2, "Maximum recursion depth")
	maxWorkers := flag.Int("workers", 5, "Number of parallel workers")
	outputDir := flag.String("output", "./downloads", "Output directory")
	httpTracking := flag.Bool("track", false, "Enable HTTP request tracking")
	convertLinks := flag.Bool("convert", true, "Convert absolute links to relative (like wget -k)")
	timeout := flag.Int("timeout", 300, "Overall timeout in seconds")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <URL>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "A modern, parallel web scraper replacement for wget-based scripts.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -depth 3 -workers 10 -track https://example.com\n", os.Args[0])
	}

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	targetURL := flag.Arg(0)

	// Create scraper
	scraper, err := NewScraper(targetURL, *maxDepth, *maxWorkers, *outputDir, *httpTracking, *convertLinks)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating scraper: %v\n", err)
		os.Exit(1)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	// Run scraper
	if err := scraper.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error running scraper: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Scraping complete!")
}
