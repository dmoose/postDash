PREFIX ?= /usr/local
BINARY = eventrelay
PLIST = com.eventrelay.plist
LAUNCH_DIR = $(HOME)/Library/LaunchAgents
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build install uninstall install-service uninstall-service restart-service upgrade status test lint fmt clean help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

build: ## Build the binary
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

install: build ## Install binary and scripts to $PREFIX
	install -d $(PREFIX)/bin
	install -m 755 $(BINARY) $(PREFIX)/bin/
	install -d $(PREFIX)/share/eventrelay/scripts
	install -m 755 scripts/er-* $(PREFIX)/share/eventrelay/scripts/
	install -d $(HOME)/.config/eventrelay
	@echo "Installed $(BINARY) to $(PREFIX)/bin/"
	@echo "Scripts: $(PREFIX)/share/eventrelay/scripts/"
	@echo "Config: ~/.config/eventrelay/eventrelay.yaml"
	@echo "Run 'make install-service' to start on login"

uninstall: uninstall-service ## Remove installed binary and scripts
	rm -f $(PREFIX)/bin/$(BINARY)
	rm -rf $(PREFIX)/share/eventrelay

install-service: install ## Install and start macOS launchd service
	install -d $(LAUNCH_DIR)
	sed 's|/usr/local/bin/eventrelay|$(PREFIX)/bin/eventrelay|g' $(PLIST) > $(LAUNCH_DIR)/$(PLIST)
	launchctl load $(LAUNCH_DIR)/$(PLIST)
	@echo "eventrelay service installed and started"
	@echo "Dashboard: http://localhost:6060"

uninstall-service: ## Stop and remove macOS launchd service
	-launchctl unload $(LAUNCH_DIR)/$(PLIST) 2>/dev/null
	rm -f $(LAUNCH_DIR)/$(PLIST)

restart-service: ## Restart the launchd service
	-launchctl unload $(LAUNCH_DIR)/$(PLIST) 2>/dev/null
	launchctl load $(LAUNCH_DIR)/$(PLIST)
	@echo "eventrelay restarted"

upgrade: build ## Build, install, and restart the running service
	@echo "Stopping eventrelay..."
	-launchctl unload $(LAUNCH_DIR)/$(PLIST) 2>/dev/null
	install -m 755 $(BINARY) $(PREFIX)/bin/
	install -d $(PREFIX)/share/eventrelay/scripts
	install -m 755 scripts/er-* $(PREFIX)/share/eventrelay/scripts/
	@if [ -f $(LAUNCH_DIR)/$(PLIST) ]; then \
		launchctl load $(LAUNCH_DIR)/$(PLIST); \
		echo "eventrelay upgraded and restarted ($(VERSION))"; \
	else \
		echo "eventrelay upgraded ($(VERSION)) — no service to restart, run manually or 'make install-service'"; \
	fi

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
