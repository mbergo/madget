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
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type Scraper struct {
	baseURL       *url.URL
	maxDepth      int
	maxWorkers    int
	visited       map[string]bool
	visitedMu     sync.RWMutex
	queue         chan Job
	wg            sync.WaitGroup
	client        *http.Client
	outputDir     string
	httpTracking  bool
	convertLinks  bool
	downloadStats Stats
	statsMu       sync.Mutex
	// Track URL to local file path mapping for link conversion
	urlToPath   map[string]string
	urlToPathMu sync.RWMutex
	htmlFiles   []string
	htmlFilesMu sync.Mutex
	cssFiles    []string
	cssFilesMu  sync.Mutex
}

type Job struct {
	url     string
	depth   int
	isAsset bool // if true, download only (no crawling for more page links)
}

type Stats struct {
	TotalRequests   int
	SuccessRequests int
	FailedRequests  int
	BytesDownloaded int64
	StartTime       time.Time
}

// cssURLRegex matches url() references in CSS content
var cssURLRegex = regexp.MustCompile(`url\(\s*['"]?([^'"\)\s]+?)['"]?\s*\)`)

// isStaticAssetURL returns true if the URL looks like a static asset (not a page to crawl)
func isStaticAssetURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	ext := strings.ToLower(path.Ext(parsedURL.Path))
	assetExts := map[string]bool{
		".css": true, ".js": true, ".mjs": true,
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".svg": true,
		".webp": true, ".ico": true, ".avif": true, ".bmp": true, ".tiff": true,
		".woff": true, ".woff2": true, ".ttf": true, ".eot": true, ".otf": true,
		".pdf": true, ".zip": true, ".tar": true, ".gz": true,
		".mp4": true, ".avi": true, ".mov": true, ".mp3": true, ".webm": true, ".ogg": true,
		".json": true, ".xml": true, ".txt": true, ".map": true,
	}
	return assetExts[ext]
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
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   60 * time.Second,
	}

	return &Scraper{
		baseURL:      parsedURL,
		maxDepth:     maxDepth,
		maxWorkers:   maxWorkers,
		visited:      make(map[string]bool),
		queue:        make(chan Job, maxWorkers*200),
		client:       client,
		outputDir:    outputDir,
		httpTracking: httpTracking,
		convertLinks: convertLinks,
		downloadStats: Stats{
			StartTime: time.Now(),
		},
		urlToPath: make(map[string]string),
		htmlFiles: make([]string, 0),
		cssFiles:  make([]string, 0),
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

func (s *Scraper) downloadPage(ctx context.Context, urlStr string, depth int, isAsset bool) error {
	if s.isVisited(urlStr) {
		return nil
	}

	s.markVisited(urlStr)

	// Update stats
	s.statsMu.Lock()
	s.downloadStats.TotalRequests++
	s.statsMu.Unlock()

	// Retry logic
	var resp *http.Response
	var lastErr error
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Create request with context
		req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers to look like a real browser
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Accept-Encoding", "identity")
		req.Header.Set("Connection", "keep-alive")

		if s.httpTracking {
			fmt.Printf("[HTTP] GET %s (depth: %d, asset: %v, attempt: %d)\n", urlStr, depth, isAsset, attempt+1)
		}

		resp, lastErr = s.client.Do(req)
		if lastErr == nil && resp.StatusCode == http.StatusOK {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		if attempt < maxRetries-1 {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	if lastErr != nil {
		s.statsMu.Lock()
		s.downloadStats.FailedRequests++
		s.statsMu.Unlock()
		return fmt.Errorf("failed to fetch %s: %w", urlStr, lastErr)
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

	// Determine content type
	contentType := resp.Header.Get("Content-Type")

	// Determine file path
	filePath := s.getFilePath(parsedURL, contentType)

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
		// Also track without trailing slash
		cleanURL := strings.TrimRight(urlStr, "/")
		if cleanURL != urlStr {
			s.urlToPath[cleanURL] = filePath
		}
		s.urlToPathMu.Unlock()

		// Track HTML files for post-processing
		if strings.Contains(contentType, "text/html") {
			s.htmlFilesMu.Lock()
			s.htmlFiles = append(s.htmlFiles, fullPath)
			s.htmlFilesMu.Unlock()
		}
		// Track CSS files for post-processing
		if strings.Contains(contentType, "text/css") {
			s.cssFilesMu.Lock()
			s.cssFiles = append(s.cssFiles, fullPath)
			s.cssFilesMu.Unlock()
		}
	}

	fmt.Printf("✓ Saved: %s (%d bytes)\n", filePath, bytesWritten)

	isHTML := strings.Contains(contentType, "text/html")
	isCSS := strings.Contains(contentType, "text/css")

	// Parse HTML and extract links if this is a page (not an asset) and within depth
	if isHTML && !isAsset && depth < s.maxDepth {
		// Re-read the file for parsing since we already consumed the body
		file.Seek(0, 0)
		links, err := s.extractLinks(file, parsedURL)
		if err != nil {
			return fmt.Errorf("failed to extract links: %w", err)
		}

		// Queue discovered URLs
		for _, link := range links {
			if s.isVisited(link) {
				continue
			}

			linkURL, err := url.Parse(link)
			if err != nil {
				continue
			}

			if s.isSameDomain(linkURL) && !isStaticAssetURL(link) {
				// Same-domain page link - crawl it for more links
				s.wg.Add(1)
				select {
				case s.queue <- Job{url: link, depth: depth + 1, isAsset: false}:
				case <-ctx.Done():
					s.wg.Done()
					return ctx.Err()
				}
			} else {
				// Static asset or cross-domain resource - download only
				s.wg.Add(1)
				select {
				case s.queue <- Job{url: link, depth: depth + 1, isAsset: true}:
				case <-ctx.Done():
					s.wg.Done()
					return ctx.Err()
				}
			}
		}
	}

	// Parse CSS files for url() references (fonts, background images, etc.)
	if isCSS {
		file.Seek(0, 0)
		cssContent, err := io.ReadAll(file)
		if err == nil {
			cssURLs := s.extractCSSURLs(string(cssContent), parsedURL)
			for _, cssURL := range cssURLs {
				if !s.isVisited(cssURL) {
					s.wg.Add(1)
					select {
					case s.queue <- Job{url: cssURL, depth: depth + 1, isAsset: true}:
					case <-ctx.Done():
						s.wg.Done()
						return ctx.Err()
					}
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

	// Only follow same domain for page crawling
	return s.isSameDomain(parsedURL)
}

// extractCSSURLs extracts all url() references from CSS content
func (s *Scraper) extractCSSURLs(cssContent string, baseURL *url.URL) []string {
	var urls []string
	seen := make(map[string]bool)

	matches := cssURLRegex.FindAllStringSubmatch(cssContent, -1)
	for _, match := range matches {
		if len(match) > 1 {
			rawURL := strings.TrimSpace(match[1])
			// Skip data URIs and empty URLs
			if rawURL == "" || strings.HasPrefix(rawURL, "data:") {
				continue
			}
			absURL := s.makeAbsoluteURL(rawURL, baseURL)
			if absURL != "" && !seen[absURL] {
				seen[absURL] = true
				urls = append(urls, absURL)
			}
		}
	}

	return urls
}

func (s *Scraper) getFilePath(parsedURL *url.URL, contentType string) string {
	urlPath := parsedURL.Path
	if urlPath == "" || urlPath == "/" {
		urlPath = "/index.html"
	}

	// If path doesn't have extension, add based on content type
	if !strings.Contains(path.Base(urlPath), ".") {
		if strings.Contains(contentType, "text/html") {
			urlPath = path.Join(urlPath, "index.html")
		} else if strings.Contains(contentType, "text/css") {
			urlPath = urlPath + ".css"
		} else if strings.Contains(contentType, "javascript") {
			urlPath = urlPath + ".js"
		} else if strings.Contains(contentType, "image/svg") {
			urlPath = urlPath + ".svg"
		} else if strings.Contains(contentType, "image/png") {
			urlPath = urlPath + ".png"
		} else if strings.Contains(contentType, "image/jpeg") {
			urlPath = urlPath + ".jpg"
		} else if strings.Contains(contentType, "image/webp") {
			urlPath = urlPath + ".webp"
		} else if strings.Contains(contentType, "font/woff2") {
			urlPath = urlPath + ".woff2"
		} else if strings.Contains(contentType, "font/woff") {
			urlPath = urlPath + ".woff"
		}
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
	seen := make(map[string]bool)

	addLink := func(rawURL string) {
		if rawURL == "" {
			return
		}
		absURL := s.makeAbsoluteURL(rawURL, baseURL)
		if absURL != "" && !seen[absURL] {
			seen[absURL] = true
			links = append(links, absURL)
		}
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Extract URLs from element attributes based on tag type
			for _, attr := range n.Attr {
				switch {
				// href attributes: <a>, <link>
				case attr.Key == "href" && (n.Data == "a" || n.Data == "link"):
					addLink(attr.Val)

				// src attributes: <script>, <img>, <source>, <video>, <audio>, <embed>, <input>, <iframe>
				case attr.Key == "src" && (n.Data == "script" || n.Data == "img" ||
					n.Data == "source" || n.Data == "video" || n.Data == "audio" ||
					n.Data == "embed" || n.Data == "input" || n.Data == "iframe"):
					addLink(attr.Val)

				// srcset attributes: <img>, <source> - contains comma-separated "url size" pairs
				case attr.Key == "srcset" && (n.Data == "img" || n.Data == "source"):
					for _, part := range strings.Split(attr.Val, ",") {
						part = strings.TrimSpace(part)
						fields := strings.Fields(part)
						if len(fields) > 0 {
							addLink(fields[0])
						}
					}

				// poster attribute: <video>
				case attr.Key == "poster" && n.Data == "video":
					addLink(attr.Val)

				// data attribute: <object>
				case attr.Key == "data" && n.Data == "object":
					addLink(attr.Val)

				// Inline style attributes with url() references
				case attr.Key == "style":
					matches := cssURLRegex.FindAllStringSubmatch(attr.Val, -1)
					for _, match := range matches {
						if len(match) > 1 {
							addLink(match[1])
						}
					}
				}
			}

			// Parse <style> tag content for url() references
			if n.Data == "style" {
				var cssContent strings.Builder
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.TextNode {
						cssContent.WriteString(c.Data)
					}
				}
				if cssContent.Len() > 0 {
					matches := cssURLRegex.FindAllStringSubmatch(cssContent.String(), -1)
					for _, match := range matches {
						if len(match) > 1 {
							addLink(match[1])
						}
					}
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
	// Skip mailto, javascript, data URIs, and fragments
	if strings.HasPrefix(link, "mailto:") || strings.HasPrefix(link, "javascript:") ||
		strings.HasPrefix(link, "#") || strings.HasPrefix(link, "data:") ||
		strings.HasPrefix(link, "tel:") || strings.HasPrefix(link, "blob:") ||
		strings.HasPrefix(link, "vbscript:") {
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
	htmlDir := filepath.Dir(htmlPath)

	// Collect all URL mappings (full URLs + path-only same-domain URLs)
	s.urlToPathMu.RLock()
	type urlMapping struct {
		urlStr     string
		targetPath string
	}
	var mappings []urlMapping
	seen := make(map[string]bool)

	for absURL, targetPath := range s.urlToPath {
		if !seen[absURL] {
			mappings = append(mappings, urlMapping{absURL, targetPath})
			seen[absURL] = true
		}
		// For same-domain URLs, also add path-only version for path-absolute references in HTML
		parsedURL, err := url.Parse(absURL)
		if err == nil && s.isSameDomain(parsedURL) && parsedURL.Path != "" && !seen[parsedURL.Path] {
			mappings = append(mappings, urlMapping{parsedURL.Path, targetPath})
			seen[parsedURL.Path] = true
		}
	}
	s.urlToPathMu.RUnlock()

	// Sort by length (longest first) to prevent partial matches
	sort.Slice(mappings, func(i, j int) bool {
		return len(mappings[i].urlStr) > len(mappings[j].urlStr)
	})

	// Two-pass replacement using placeholders to prevent cascade effects
	// Pass 1: Replace all URL occurrences with unique placeholders
	type replacement struct {
		placeholder string
		relPath     string
	}
	var replacements []replacement
	modified := false

	for i, m := range mappings {
		if !strings.Contains(htmlStr, m.urlStr) {
			continue
		}

		targetFullPath := filepath.Join(s.outputDir, m.targetPath)
		relPath, err := filepath.Rel(htmlDir, targetFullPath)
		if err != nil {
			continue
		}
		relPath = filepath.ToSlash(relPath)

		placeholder := fmt.Sprintf("\x00MADGET_%d\x00", i)
		htmlStr = strings.ReplaceAll(htmlStr, m.urlStr, placeholder)
		replacements = append(replacements, replacement{placeholder, relPath})
		modified = true
	}

	// Pass 2: Replace placeholders with actual relative paths
	for _, r := range replacements {
		htmlStr = strings.ReplaceAll(htmlStr, r.placeholder, r.relPath)
	}

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

	// Convert HTML files
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

	// Convert CSS files
	s.cssFilesMu.Lock()
	cssFiles := make([]string, len(s.cssFiles))
	copy(cssFiles, s.cssFiles)
	s.cssFilesMu.Unlock()

	cssConverted := 0
	for _, cssFile := range cssFiles {
		if err := s.convertLinksInCSS(cssFile); err != nil {
			fmt.Printf("✗ Failed to convert links in %s: %v\n", cssFile, err)
		} else {
			cssConverted++
		}
	}

	fmt.Printf("✓ Converted links in %d CSS files\n", cssConverted)

	return nil
}

// convertLinksInCSS converts absolute URLs in CSS files to relative paths
func (s *Scraper) convertLinksInCSS(cssPath string) error {
	content, err := os.ReadFile(cssPath)
	if err != nil {
		return fmt.Errorf("failed to read CSS file: %w", err)
	}

	cssStr := string(content)
	cssDir := filepath.Dir(cssPath)

	// Collect all URL mappings (full URLs + path-only same-domain URLs)
	s.urlToPathMu.RLock()
	type urlMapping struct {
		urlStr     string
		targetPath string
	}
	var mappings []urlMapping
	seen := make(map[string]bool)

	for absURL, targetPath := range s.urlToPath {
		if !seen[absURL] {
			mappings = append(mappings, urlMapping{absURL, targetPath})
			seen[absURL] = true
		}
		parsedURL, err := url.Parse(absURL)
		if err == nil && s.isSameDomain(parsedURL) && parsedURL.Path != "" && !seen[parsedURL.Path] {
			mappings = append(mappings, urlMapping{parsedURL.Path, targetPath})
			seen[parsedURL.Path] = true
		}
	}
	s.urlToPathMu.RUnlock()

	sort.Slice(mappings, func(i, j int) bool {
		return len(mappings[i].urlStr) > len(mappings[j].urlStr)
	})

	// Two-pass placeholder replacement
	type replacement struct {
		placeholder string
		relPath     string
	}
	var replacements []replacement
	modified := false

	for i, m := range mappings {
		if !strings.Contains(cssStr, m.urlStr) {
			continue
		}

		targetFullPath := filepath.Join(s.outputDir, m.targetPath)
		relPath, err := filepath.Rel(cssDir, targetFullPath)
		if err != nil {
			continue
		}
		relPath = filepath.ToSlash(relPath)

		placeholder := fmt.Sprintf("\x00MADGET_%d\x00", i)
		cssStr = strings.ReplaceAll(cssStr, m.urlStr, placeholder)
		replacements = append(replacements, replacement{placeholder, relPath})
		modified = true
	}

	for _, r := range replacements {
		cssStr = strings.ReplaceAll(cssStr, r.placeholder, r.relPath)
	}

	if modified {
		if err := os.WriteFile(cssPath, []byte(cssStr), 0644); err != nil {
			return fmt.Errorf("failed to write modified CSS: %w", err)
		}
	}

	return nil
}

func (s *Scraper) worker(ctx context.Context, id int) {
	for {
		select {
		case job, ok := <-s.queue:
			if !ok {
				return
			}

			if err := s.downloadPage(ctx, job.url, job.depth, job.isAsset); err != nil {
				if s.httpTracking {
					fmt.Printf("✗ Worker %d error: %v\n", id, err)
				}
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

	// Start workers (no WaitGroup tracking for workers - only jobs are tracked)
	for i := 0; i < s.maxWorkers; i++ {
		go s.worker(ctx, i)
	}

	// Add initial job
	s.wg.Add(1)
	s.queue <- Job{url: s.baseURL.String(), depth: 0, isAsset: false}

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
	maxDepth := flag.Int("depth", 5, "Maximum recursion depth")
	maxWorkers := flag.Int("workers", 10, "Number of parallel workers")
	outputDir := flag.String("output", "./downloads", "Output directory")
	httpTracking := flag.Bool("track", false, "Enable HTTP request tracking")
	convertLinks := flag.Bool("convert", true, "Convert absolute links to relative (like wget -k)")
	timeout := flag.Int("timeout", 600, "Overall timeout in seconds")

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
