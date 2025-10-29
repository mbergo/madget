# Web Scraper - The Calm Successor to wgetNervoso.sh

A modern, parallel web scraper written in Go that replaces the anxiety-ridden wget script with zen-like efficiency.

## What Happened to the Old Script?

Your original `wgetNervoso.sh` was essentially:
```bash
wget -D `echo $2|cut -d/ -f1` -t 3 --header 'User-Agent: ...' \
  --no-check-certificate -r -E -H -k -K -p -e robots=off -l $1 $2
```

**Problems:**
- Single-threaded (downloads one file at a time)
- Limited error handling
- No download statistics
- Fragile URL parsing with command substitution
- No progress tracking beyond wget's basic output
- No graceful timeout handling

## What This Does Better

✅ **Parallel Processing**: Configurable worker pool (default 5 workers)  
✅ **HTTP Tracking**: See every request with status codes and sizes  
✅ **Robust Error Handling**: Continues on errors, reports stats  
✅ **Smart Link Extraction**: HTML parsing with proper URL resolution  
✅ **Download Statistics**: Track speed, success rate, total bytes  
✅ **Context-aware**: Proper timeout and cancellation handling  
✅ **Type Safety**: Go's type system prevents entire classes of bugs  
✅ **Memory Efficient**: Streams files to disk, doesn't load in memory  

## Installation

### Prerequisites
- Go 1.21 or later

### Build
```bash
# Get the dependency
go mod download

# Build the binary
go build -o web-scraper web-scraper.go

# Or build with optimizations
go build -ldflags="-s -w" -o web-scraper web-scraper.go
```

### Quick Install
```bash
# One-liner to build and install to /usr/local/bin
go build -o web-scraper web-scraper.go && sudo mv web-scraper /usr/local/bin/
```

## Usage

### Basic Usage
```bash
# Scrape a website (equivalent to old: ./wgetNervoso.sh 2 https://example.com)
./web-scraper -depth 2 https://example.com
```

### With HTTP Tracking
```bash
# See every HTTP request and response
./web-scraper -depth 3 -track https://example.com
```

### High-Performance Scraping
```bash
# Use 20 parallel workers for faster downloads
./web-scraper -depth 5 -workers 20 https://example.com
```

### Custom Output Directory
```bash
# Save to a specific directory
./web-scraper -output ./my-downloads -depth 2 https://example.com
```

### With Timeout
```bash
# Set overall timeout to 10 minutes (600 seconds)
./web-scraper -timeout 600 -depth 3 https://example.com
```

### All Options Combined
```bash
./web-scraper \
  -depth 4 \
  -workers 15 \
  -output ./website-backup \
  -track \
  -timeout 1800 \
  https://example.com
```

## Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-depth` | 2 | Maximum recursion depth (like `$1` in old script) |
| `-workers` | 5 | Number of parallel download workers |
| `-output` | ./downloads | Directory to save downloaded files |
| `-track` | false | Enable HTTP request/response tracking |
| `-convert` | true | Convert absolute links to relative (like wget `-k`) |
| `-timeout` | 300 | Overall timeout in seconds (5 minutes) |

## Comparison to Original Script

| Feature | wgetNervoso.sh | web-scraper.go |
|---------|----------------|----------------|
| Parallel Downloads | ❌ Single-threaded | ✅ Configurable workers |
| HTTP Tracking | ❌ Basic wget output | ✅ Detailed request/response logs |
| Error Recovery | ⚠️ Retries 3 times per file | ✅ Continues on errors, reports stats |
| Download Stats | ❌ None | ✅ Speed, size, success rate |
| Timeout Control | ❌ Only per-request | ✅ Global + per-request |
| Resource Usage | 🐌 Sequential I/O | 🚀 Efficient parallel I/O |
| Code Maintenance | 💀 Shell escaping hell | ✨ Type-safe, readable Go |

## Output Example

```
🚀 Starting scraper...
   Base URL: https://example.com
   Max Depth: 3
   Workers: 10
   Output: ./downloads
   HTTP Tracking: true

[HTTP] GET https://example.com (depth: 0)
[HTTP] 200 https://example.com (Content-Length: 15432)
✓ Saved: example.com/index.html (15432 bytes)

[HTTP] GET https://example.com/about (depth: 1)
[HTTP] 200 https://example.com/about (Content-Length: 8234)
✓ Saved: example.com/about/index.html (8234 bytes)

...

🔗 Converting absolute links to relative...
✓ Converted links in 143 HTML files

📊 Download Statistics:
   Duration: 12.3s
   Total Requests: 147
   Successful: 143
   Failed: 4
   Bytes Downloaded: 3.2 MB
   Average Speed: 267.2 KB/s

✅ Scraping complete!
```

## Features Explained

### Parallel Processing
Multiple workers download different files simultaneously. This is especially effective for sites with many small files (images, CSS, JS).

### Smart Link Extraction
Parses HTML properly using Go's `golang.org/x/net/html` package. Resolves relative URLs, handles fragments, and avoids common pitfalls like JavaScript URLs.

### Domain Restriction
Automatically stays within the same domain as the target URL (like the old `-D` flag).

### File Organization
Creates directory structure matching the website:
```
downloads/
└── example.com/
    ├── index.html
    ├── about/
    │   └── index.html
    └── assets/
        ├── style.css
        └── script.js
```

### Link Conversion (The `-k` Magic)
**Enabled by default** - this is the feature that makes downloaded sites actually browseable offline!

After downloading, the scraper automatically converts all absolute URLs to relative paths:

**Before conversion:**
```html
<a href="https://example.com/about.html">About</a>
<img src="https://example.com/logo.png">
```

**After conversion:**
```html
<a href="./about.html">About</a>
<img src="./logo.png">
```

This means you can:
- Open `index.html` directly in your browser
- Browse the entire site offline
- Move the download folder anywhere
- Archive sites for long-term storage

To disable link conversion (keep absolute URLs):
```bash
./web-scraper -convert=false https://example.com
```

The conversion happens **after all downloads complete**, so you get:
1. Fast parallel downloading (no performance penalty)
2. Complete URL-to-file mapping (knows every downloaded resource)
3. Accurate relative paths (calculated from each HTML file's location)

This replicates wget's `-k` (`--convert-links`) flag but with better handling of:
- Parallel downloads (no race conditions)
- Nested directory structures
- Cross-platform paths (Windows backslashes → URL slashes)

### Graceful Shutdown
Handles Ctrl+C properly, waits for in-flight downloads to complete.

## Advanced Usage

### As a Library
You can import and use the scraper in your own Go programs:

```go
import "web-scraper"

func main() {
    scraper, err := NewScraper(
        "https://example.com",
        3,     // max depth
        10,    // workers
        "./output",
        true,  // HTTP tracking
    )
    if err != nil {
        log.Fatal(err)
    }
    
    ctx := context.Background()
    scraper.Run(ctx)
}
```

### Custom Filtering
Edit the `shouldFollow()` method to add custom URL filtering logic:

```go
func (s *Scraper) shouldFollow(urlStr string) bool {
    // Only follow blog posts
    return strings.Contains(urlStr, "/blog/")
}
```

## Performance Tips

1. **Adjust Workers**: Start with 5-10 workers. Increase for bandwidth-rich connections, decrease for slow sites or to be polite.

2. **Use Depth Wisely**: Each depth level exponentially increases pages. Depth 2-3 is usually sufficient.

3. **Monitor HTTP Tracking**: Use `-track` to see if you're hitting rate limits or errors.

4. **Set Reasonable Timeouts**: Default 5 minutes is usually plenty. Increase for huge sites.

## Troubleshooting

### "Too many open files" error
Increase your system's file descriptor limit:
```bash
ulimit -n 4096
```

### Site blocks the scraper
Some sites detect scrapers. The tool uses a realistic Chrome User-Agent, but some sites may still block you. Consider:
- Reducing worker count
- Adding delays between requests (requires code modification)
- Using a proxy

### TLS certificate errors
The tool skips certificate verification (like `--no-check-certificate`). If you want strict verification, edit the code to set `InsecureSkipVerify: false`.

## Ethical Considerations

Like the original script's `robots=off` flag, this tool ignores robots.txt. Please use responsibly:

- ✅ Scrape your own content
- ✅ Scrape public data with permission
- ✅ Respect rate limits and server load
- ❌ Don't hammer servers with 100 workers
- ❌ Don't scrape private/authenticated content
- ❌ Don't ignore explicit "no scraping" policies

## License

MIT License - Do whatever you want with it. No anxiety required.

## Migration from wgetNervoso.sh

Old command:
```bash
./wgetNervoso.sh 3 https://example.com/blog
```

New equivalent:
```bash
./web-scraper -depth 3 https://example.com/blog
```

For exactly the same behavior but faster:
```bash
./web-scraper -depth 3 -workers 10 https://example.com/blog
```

## Future Enhancements

Possible additions (PRs welcome!):
- [ ] Respect robots.txt option
- [ ] Rate limiting per domain
- [ ] Resume interrupted downloads
- [ ] Export sitemap
- [ ] Compress output
- [ ] Docker container
- [ ] Progress bar for terminal UI
- [ ] Prometheus metrics export

---

Built with ❤️ and significantly less nervousness than the original script.
