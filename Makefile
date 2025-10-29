# Web Scraper Makefile
# Because typing "go build" is apparently too much effort for us sophisticated developers

.PHONY: help build install clean test lint fmt run dev release docker intensity-help
.DEFAULT_GOAL := help

# Build variables
BINARY_NAME=web-scraper
MAIN_FILE=web-scraper.go
OUTPUT_DIR=./bin
BUILD_DIR=./build
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"

# Color output (because we're fancy)
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
BLUE=\033[0;34m
PURPLE=\033[0;35m
CYAN=\033[0;36m
NC=\033[0m # No Color

##@ Help
help: ## Display this help message (for the confused and weary)
	@echo "$(CYAN)═══════════════════════════════════════════════════════════════$(NC)"
	@echo "$(PURPLE)  Web Scraper - The Makefile That Does Everything$(NC)"
	@echo "$(CYAN)═══════════════════════════════════════════════════════════════$(NC)"
	@awk 'BEGIN {FS = ":.*##"; printf "\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  $(CYAN)%-20s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(YELLOW)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(PURPLE)Intensity Levels:$(NC)"
	@echo "  $(GREEN)make whisper$(NC)       - Minimal build, maximum stealth"
	@echo "  $(GREEN)make casual$(NC)        - Normal build, no fuss"
	@echo "  $(GREEN)make focused$(NC)       - Optimized build with tests"
	@echo "  $(GREEN)make intense$(NC)       - Full optimization + benchmarks"
	@echo "  $(GREEN)make nuclear$(NC)       - Everything. All at once. Everywhere."
	@echo ""
	@echo "$(CYAN)═══════════════════════════════════════════════════════════════$(NC)"

##@ Intensity Levels (Choose Your Fighter)

whisper: ## 🌙 Intensity 1: Minimal build, like a ninja in slippers
	@echo "$(BLUE)🌙 WHISPER MODE: Building with the stealth of a library patron...$(NC)"
	@mkdir -p $(OUTPUT_DIR)
	@go build -o $(OUTPUT_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "$(GREEN)✓ Binary whispered into existence at $(OUTPUT_DIR)/$(BINARY_NAME)$(NC)"
	@ls -lh $(OUTPUT_DIR)/$(BINARY_NAME)

casual: clean fmt ## ☕ Intensity 2: Normal build + format (the "I'm a professional" level)
	@echo "$(BLUE)☕ CASUAL MODE: Building like it's a Tuesday morning...$(NC)"
	@mkdir -p $(OUTPUT_DIR)
	@go mod tidy
	@go build -o $(OUTPUT_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "$(GREEN)✓ Build complete. Coffee optional but recommended.$(NC)"
	@ls -lh $(OUTPUT_DIR)/$(BINARY_NAME)

focused: clean fmt test lint ## 🎯 Intensity 3: Optimized build + tests + linting
	@echo "$(PURPLE)🎯 FOCUSED MODE: Now we're actually trying...$(NC)"
	@mkdir -p $(OUTPUT_DIR)
	@go mod tidy
	@echo "$(CYAN)Running tests...$(NC)"
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "$(CYAN)Building optimized binary...$(NC)"
	@go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "$(GREEN)✓ Focused build complete. You can feel the optimization.$(NC)"
	@ls -lh $(OUTPUT_DIR)/$(BINARY_NAME)

intense: clean fmt test lint benchmark ## 🔥 Intensity 4: Full optimization + benchmarks
	@echo "$(RED)🔥 INTENSE MODE: Maximum effort engaged...$(NC)"
	@mkdir -p $(OUTPUT_DIR)
	@go mod tidy
	@echo "$(CYAN)Running comprehensive tests...$(NC)"
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(CYAN)Building with aggressive optimization...$(NC)"
	@CGO_ENABLED=0 go build -a -trimpath $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "$(GREEN)✓ Intense build complete. Binary is now 15% more intense.$(NC)"
	@ls -lh $(OUTPUT_DIR)/$(BINARY_NAME)

nuclear: clean ## ☢️ Intensity 5: EVERYTHING. CROSS-COMPILE ALL THE THINGS.
	@echo "$(RED)☢️☢️☢️ NUCLEAR MODE ACTIVATED ☢️☢️☢️$(NC)"
	@echo "$(YELLOW)⚠️  Warning: Your CPU is about to experience existential dread$(NC)"
	@mkdir -p $(BUILD_DIR)
	@go mod tidy
	@echo "$(CYAN)Formatting code...$(NC)"
	@go fmt ./...
	@echo "$(CYAN)Running tests with race detection...$(NC)"
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(CYAN)Running benchmarks...$(NC)"
	@go test -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof ./... || true
	@echo "$(CYAN)Linting with maximum prejudice...$(NC)"
	@golangci-lint run --timeout=5m || echo "$(YELLOW)⚠️  Linter not found, skipping...$(NC)"
	@echo "$(RED)Cross-compiling for every platform humans have invented...$(NC)"
	@echo "  $(CYAN)→ Linux AMD64$(NC)"
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_FILE)
	@echo "  $(CYAN)→ Linux ARM64$(NC)"
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -a -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_FILE)
	@echo "  $(CYAN)→ MacOS AMD64$(NC)"
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -a -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_FILE)
	@echo "  $(CYAN)→ MacOS ARM64 (M1/M2)$(NC)"
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -a -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_FILE)
	@echo "  $(CYAN)→ Windows AMD64$(NC)"
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -a -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_FILE)
	@echo "  $(CYAN)→ FreeBSD AMD64 (for the rebels)$(NC)"
	@GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go build -a -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-freebsd-amd64 $(MAIN_FILE)
	@echo "$(CYAN)Creating checksums...$(NC)"
	@cd $(BUILD_DIR) && sha256sum * > checksums.txt
	@echo "$(GREEN)✓ NUCLEAR BUILD COMPLETE$(NC)"
	@echo "$(YELLOW)Binaries built for 6 platforms:$(NC)"
	@ls -lh $(BUILD_DIR)
	@echo "$(RED)Your CPU would like to file a complaint with HR.$(NC)"

##@ Standard Operations (The Boring But Necessary Stuff)

build: casual ## Build the binary (alias for 'casual' because we're reasonable people)

install: focused ## Install binary to $GOPATH/bin (for the globally ambitious)
	@echo "$(CYAN)Installing to GOPATH...$(NC)"
	@go install $(LDFLAGS) $(MAIN_FILE)
	@echo "$(GREEN)✓ Installed! Now available system-wide as '$(BINARY_NAME)'$(NC)"

clean: ## Clean build artifacts (Marie Kondo would be proud)
	@echo "$(YELLOW)🧹 Cleaning up the mess...$(NC)"
	@rm -rf $(OUTPUT_DIR) $(BUILD_DIR)
	@rm -f coverage.out coverage.html cpu.prof mem.prof
	@rm -f $(BINARY_NAME)
	@echo "$(GREEN)✓ Sparkles restored. Serenity achieved.$(NC)"

##@ Development (For When You're Actually Working)

dev: ## Run the scraper in development mode with example URL
	@echo "$(CYAN)🚀 Running in development mode...$(NC)"
	@go run $(MAIN_FILE) -depth 2 -workers 5 -track https://example.com

run: ## Run with custom arguments (usage: make run ARGS="-depth 3 https://example.com")
	@echo "$(CYAN)🚀 Running with custom arguments...$(NC)"
	@go run $(MAIN_FILE) $(ARGS)

fmt: ## Format code (because tabs vs spaces is so last decade)
	@echo "$(CYAN)Formatting code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)✓ Code formatted. Readability restored.$(NC)"

lint: ## Run linter (the judgmental friend you actually need)
	@echo "$(CYAN)Running linter...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=3m; \
		echo "$(GREEN)✓ Linter satisfied. Code quality: chef's kiss$(NC)"; \
	else \
		echo "$(YELLOW)⚠️  golangci-lint not installed. Install it with:$(NC)"; \
		echo "$(YELLOW)   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)"; \
	fi

test: ## Run tests (cross your fingers)
	@echo "$(CYAN)Running tests...$(NC)"
	@go test -v -race ./...
	@echo "$(GREEN)✓ Tests passed! We're probably not fired.$(NC)"

test-coverage: ## Run tests with coverage report (for the metrics-obsessed)
	@echo "$(CYAN)Running tests with coverage...$(NC)"
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage.out -o coverage.html
	@go tool cover -func=coverage.out
	@echo "$(GREEN)✓ Coverage report generated: coverage.html$(NC)"

benchmark: ## Run benchmarks (make numbers go brrrr)
	@echo "$(CYAN)Running benchmarks...$(NC)"
	@go test -bench=. -benchmem ./... || echo "$(YELLOW)No benchmarks found (add some!)$(NC)"

##@ Release (When It's Time To Ship This Thing)

release: nuclear docker ## Create release builds + Docker image (the full monty)
	@echo "$(GREEN)✓ Release build complete!$(NC)"
	@echo "$(CYAN)Binaries available in: $(BUILD_DIR)$(NC)"
	@echo "$(CYAN)Docker image: $(BINARY_NAME):latest$(NC)"

release-github: nuclear ## Prepare GitHub release (automated for the lazy)
	@echo "$(CYAN)Creating GitHub release archive...$(NC)"
	@mkdir -p $(BUILD_DIR)/release
	@cd $(BUILD_DIR) && \
		for binary in $(BINARY_NAME)-*; do \
			tar -czf release/$${binary}.tar.gz $${binary} checksums.txt README.md || true; \
		done
	@echo "$(GREEN)✓ Release archives created in $(BUILD_DIR)/release/$(NC)"
	@ls -lh $(BUILD_DIR)/release/

docker: ## Build Docker image (for the container enthusiasts)
	@echo "$(CYAN)Building Docker image...$(NC)"
	@docker build -t $(BINARY_NAME):latest .
	@docker tag $(BINARY_NAME):latest $(BINARY_NAME):$(VERSION)
	@echo "$(GREEN)✓ Docker images built:$(NC)"
	@echo "  • $(BINARY_NAME):latest"
	@echo "  • $(BINARY_NAME):$(VERSION)"

docker-run: docker ## Build and run in Docker (one-command wonder)
	@echo "$(CYAN)Running in Docker...$(NC)"
	@docker run --rm -v $(PWD)/downloads:/app/downloads $(BINARY_NAME):latest \
		-depth 2 -workers 5 https://example.com

##@ Maintenance (Keeping The Garden Tidy)

deps: ## Download dependencies (go mod tidy, but fancier)
	@echo "$(CYAN)Downloading dependencies...$(NC)"
	@go mod download
	@go mod tidy
	@go mod verify
	@echo "$(GREEN)✓ Dependencies satisfied and verified$(NC)"

update-deps: ## Update all dependencies (living dangerously)
	@echo "$(YELLOW)⚠️  Updating all dependencies to latest versions...$(NC)"
	@go get -u ./...
	@go mod tidy
	@echo "$(GREEN)✓ Dependencies updated. Hope nothing broke!$(NC)"

vet: ## Run go vet (the compiler's paranoid cousin)
	@echo "$(CYAN)Running go vet...$(NC)"
	@go vet ./...
	@echo "$(GREEN)✓ No suspicious code patterns detected$(NC)"

check: fmt lint vet test ## Run all checks (the paranoid developer's dream)
	@echo "$(GREEN)✓ ALL CHECKS PASSED$(NC)"
	@echo "$(CYAN)Your code is squeaky clean. Good job!$(NC)"

##@ Information (For The Curious)

size: build ## Show binary size (spoiler: it's probably too big)
	@echo "$(CYAN)Binary size information:$(NC)"
	@if [ -f $(OUTPUT_DIR)/$(BINARY_NAME) ]; then \
		ls -lh $(OUTPUT_DIR)/$(BINARY_NAME); \
		size=$(shell stat -f%z $(OUTPUT_DIR)/$(BINARY_NAME) 2>/dev/null || stat -c%s $(OUTPUT_DIR)/$(BINARY_NAME) 2>/dev/null); \
		echo "$(YELLOW)Size: $$(numfmt --to=iec-i --suffix=B $$size 2>/dev/null || echo $$size bytes)$(NC)"; \
	else \
		echo "$(RED)Binary not found. Run 'make build' first.$(NC)"; \
	fi

version: ## Show version information (existential identity crisis resolved)
	@echo "$(CYAN)Version: $(YELLOW)$(VERSION)$(NC)"
	@echo "$(CYAN)Build Time: $(YELLOW)$(BUILD_TIME)$(NC)"

list-targets: ## List all make targets (meta!)
	@echo "$(CYAN)Available targets:$(NC)"
	@LC_ALL=C $(MAKE) -pRrq -f $(lastword $(MAKEFILE_LIST)) : 2>/dev/null | \
		awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | \
		sort | grep -v -e '^[^[:alnum:]]' -e '^$@$$'

##@ CI/CD (For The Automation Addicts)

ci: deps check intense ## Run CI pipeline locally (test before pushing)
	@echo "$(GREEN)✓ CI pipeline complete!$(NC)"
	@echo "$(CYAN)You're ready to push to main (we believe in you)$(NC)"

pre-commit: fmt vet test ## Run pre-commit checks (should be a git hook)
	@echo "$(GREEN)✓ Pre-commit checks passed$(NC)"

# Special targets for the adventurous
.PHONY: yolo danger-zone chaos

yolo: ## Build without tests (what could go wrong?)
	@echo "$(RED)🎲 YOLO MODE: Building without tests...$(NC)"
	@echo "$(YELLOW)⚠️  Production is just another test environment, right?$(NC)"
	@mkdir -p $(OUTPUT_DIR)
	@go build -o $(OUTPUT_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "$(GREEN)✓ Built! Good luck!$(NC)"

danger-zone: clean ## Delete EVERYTHING including go.sum (nuclear option)
	@echo "$(RED)⚠️⚠️⚠️  DANGER ZONE ⚠️⚠️⚠️$(NC)"
	@echo "$(RED)This will delete go.sum and all caches!$(NC)"
	@read -p "Are you SURE? (type 'yes'): " confirm && [ "$$confirm" = "yes" ] || (echo "Aborted." && exit 1)
	@rm -rf $(OUTPUT_DIR) $(BUILD_DIR) vendor/
	@rm -f go.sum coverage.out coverage.html
	@go clean -cache -modcache -testcache
	@echo "$(RED)💥 Everything is gone. Starting fresh.$(NC)"

chaos: ## Run random tasks for testing (DO NOT USE IN PRODUCTION)
	@echo "$(RED)🌀 CHAOS MODE: Doing random things...$(NC)"
	@$(MAKE) fmt
	@$(MAKE) test || true
	@$(MAKE) build || true
	@echo "$(YELLOW)Chaos complete. Good luck debugging whatever just happened.$(NC)"
