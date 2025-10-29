# Quick Start Guide
## For People Who Don't Read Documentation (We Understand)

### 🚀 TL;DR

```bash
# Build
make casual

# Run
./bin/web-scraper -depth 3 https://example.com

# Done. Go home.
```

---

## The Intensity Scale (Choose Your Adventure)

### 🌙 Whisper - "I just want a binary"
```bash
make whisper
```
**Result**: Basic binary. No frills. Like toast without butter.

---

### ☕ Casual - "I'm a professional" (Recommended)
```bash
make casual
```
**Result**: 
- Formatted code ✓
- Built binary ✓
- Your dignity intact ✓

**Use it:**
```bash
./bin/web-scraper -depth 2 -workers 5 https://example.com
```

---

### 🎯 Focused - "Actually trying now"
```bash
make focused
```
**Result**: 
- Tests run ✓
- Code linted ✓
- Optimized binary ✓
- Coverage report generated ✓
- Feeling accomplished ✓

---

### 🔥 Intense - "Maximum effort"
```bash
make intense
```
**Result**:
- Everything from Focused
- Benchmarks run ✓
- Aggressive optimizations ✓
- Coverage HTML report ✓
- Binary size reduced ✓
- Your CPU is warm ✓

---

### ☢️ Nuclear - "DEFCON 1"
```bash
make nuclear
```
**Result**:
- Cross-compiled for 6 platforms
- Docker image built (if you have Docker)
- Checksums generated
- All tests + benchmarks
- Your CPU filing a restraining order
- Coffee addiction intensifies

**Find your binaries in:** `./build/`

```
build/
├── web-scraper-linux-amd64
├── web-scraper-linux-arm64
├── web-scraper-darwin-amd64
├── web-scraper-darwin-arm64
├── web-scraper-windows-amd64.exe
├── web-scraper-freebsd-amd64
└── checksums.txt
```

---

## Common Usage Examples

### Basic scraping
```bash
./bin/web-scraper https://example.com
```

### Deeper scraping with more workers
```bash
./bin/web-scraper -depth 5 -workers 20 https://example.com
```

### With HTTP tracking (see every request)
```bash
./bin/web-scraper -depth 3 -track https://example.com
```

### Custom output directory
```bash
./bin/web-scraper -output ./my-site -depth 3 https://example.com
```

### Without link conversion (keep absolute URLs)
```bash
./bin/web-scraper -convert=false https://example.com
```

### Everything at once
```bash
./bin/web-scraper \
  -depth 5 \
  -workers 15 \
  -output ./backup \
  -track \
  -timeout 1800 \
  https://example.com
```

---

## Docker Usage

### Quick Docker run
```bash
make docker-run
```

### Manual Docker
```bash
# Build image
docker build -t web-scraper .

# Run it
docker run --rm -v $(pwd)/downloads:/app/downloads \
  web-scraper -depth 3 -workers 10 https://example.com
```

---

## Development Workflow

### Format code
```bash
make fmt
```

### Run tests
```bash
make test
```

### Run with custom args during development
```bash
make run ARGS="-depth 3 -track https://example.com"
```

### Check everything before committing
```bash
make check
```

---

## Installation (System-wide)

```bash
make install
```

Now you can use `web-scraper` from anywhere:
```bash
web-scraper -depth 3 https://example.com
```

---

## Troubleshooting

### "make: command not found"
You don't have make installed. Just build manually:
```bash
go build -o web-scraper web-scraper.go
```

### "golangci-lint not found"
Install it:
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

Or skip linting by using `make whisper` or `make casual`

### "Too many open files"
Increase file descriptor limit:
```bash
ulimit -n 4096
```

### Binary is huge
Use optimization:
```bash
make intense
```

Or manually:
```bash
go build -ldflags="-s -w" -o web-scraper web-scraper.go
upx web-scraper  # if you have upx installed
```

---

## Migration from wgetNervoso.sh

**Old:**
```bash
./wgetNervoso.sh 3 https://example.com
```

**New:**
```bash
make casual
./bin/web-scraper -depth 3 https://example.com
```

**Even Better:**
```bash
make casual
./bin/web-scraper -depth 3 -workers 10 -track https://example.com
```

---

## Cheat Sheet

| Command | What It Does | Time |
|---------|--------------|------|
| `make whisper` | Just build | ~3s |
| `make casual` | Build + format | ~5s |
| `make focused` | Build + test + lint | ~15s |
| `make intense` | Full optimization | ~30s |
| `make nuclear` | EVERYTHING | ~2min |
| `make clean` | Delete all builds | ~1s |
| `make help` | Show all options | ~0s |

---

## Pro Tips

1. **Start with `casual`** - It's the sweet spot
2. **Use `-track`** to debug issues
3. **Increase workers** for faster downloads (try 10-20)
4. **Decrease depth** if scraping takes forever
5. **Check `downloads/` directory** for output
6. **Use `make nuclear`** for release builds
7. **Run `make check`** before committing
8. **Use Docker** if you want isolation

---

## One-Liners for the Impatient

```bash
# Build and run in one command
make casual && ./bin/web-scraper -depth 3 https://example.com

# Build, install, and use system-wide
make install && web-scraper -depth 3 https://example.com

# Nuclear build and package for release
make nuclear && cd build && ls -lh

# Clean everything and rebuild
make clean && make focused

# Run in Docker without installing anything
docker build -t ws . && docker run --rm -v $(pwd)/dl:/app/downloads ws -depth 2 https://example.com
```

---

## When Things Go Wrong

**Build fails?**
```bash
go mod tidy
make clean
make casual
```

**Tests fail?**
```bash
# Just YOLO it
make yolo
```

**Everything is broken?**
```bash
make danger-zone  # Nuclear reset (deletes EVERYTHING)
make deps         # Restore dependencies
make casual       # Try again
```

**Still broken?**
```bash
# The classic
rm -rf vendor/ go.sum
go mod tidy
go mod download
make casual
```

**STILL broken?**

Blame Mercury being in retrograde and try again tomorrow.

---

## Summary

- **Just want to use it?** → `make casual` → `./bin/web-scraper URL`
- **Want to develop?** → `make focused`
- **Want to release?** → `make nuclear`
- **Want to Docker?** → `make docker-run`
- **Want to understand everything?** → Read README.md
- **Want to give up?** → Don't. We believe in you.

---

**Remember:** The best build is the one that works for you. Start simple, scale up as needed.

Now go forth and scrape responsibly! 🚀
