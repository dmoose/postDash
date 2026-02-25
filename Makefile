PREFIX ?= /usr/local
BINARY = eventrelay
PLIST = com.eventrelay.plist
LAUNCH_DIR = $(HOME)/Library/LaunchAgents

.PHONY: build install uninstall install-service uninstall-service status test lint fmt clean help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

build: ## Build the binary
	go build -o $(BINARY) .

install: build ## Install binary to $PREFIX/bin
	install -d $(PREFIX)/bin
	install -m 755 $(BINARY) $(PREFIX)/bin/
	install -d $(HOME)/.config/eventrelay
	@echo "Installed $(BINARY) to $(PREFIX)/bin/"
	@echo "Config: ~/.config/eventrelay/eventrelay.yaml"
	@echo "Run 'make install-service' to start on login"

uninstall: uninstall-service ## Remove installed binary
	rm -f $(PREFIX)/bin/$(BINARY)

install-service: install ## Install and start macOS launchd service
	install -d $(LAUNCH_DIR)
	sed 's|/usr/local/bin/eventrelay|$(PREFIX)/bin/eventrelay|g' $(PLIST) > $(LAUNCH_DIR)/$(PLIST)
	launchctl load $(LAUNCH_DIR)/$(PLIST)
	@echo "eventrelay service installed and started"
	@echo "Dashboard: http://localhost:6060"

uninstall-service: ## Stop and remove macOS launchd service
	-launchctl unload $(LAUNCH_DIR)/$(PLIST) 2>/dev/null
	rm -f $(LAUNCH_DIR)/$(PLIST)

status: ## Check if eventrelay is running
	@$(PREFIX)/bin/$(BINARY) --status 2>/dev/null || ./$(BINARY) --status 2>/dev/null || echo "eventrelay not installed"

test: ## Run all tests with race detector
	go test -race ./...

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format Go source files
	gofmt -w .

clean: ## Remove build artifacts
	rm -f $(BINARY)
