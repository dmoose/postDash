PREFIX ?= /usr/local
BINARY = eventrelay
PLIST = com.eventrelay.plist
LAUNCH_DIR = $(HOME)/Library/LaunchAgents

.PHONY: build install uninstall install-service uninstall-service status test clean

build:
	go build -o $(BINARY) .

install: build
	install -d $(PREFIX)/bin
	install -m 755 $(BINARY) $(PREFIX)/bin/
	install -d $(HOME)/.config/eventrelay
	@echo "Installed $(BINARY) to $(PREFIX)/bin/"
	@echo "Config: ~/.config/eventrelay/eventrelay.yaml"
	@echo "Run 'make install-service' to start on login"

uninstall: uninstall-service
	rm -f $(PREFIX)/bin/$(BINARY)

install-service: install
	install -d $(LAUNCH_DIR)
	sed 's|/usr/local/bin/eventrelay|$(PREFIX)/bin/eventrelay|g' $(PLIST) > $(LAUNCH_DIR)/$(PLIST)
	launchctl load $(LAUNCH_DIR)/$(PLIST)
	@echo "eventrelay service installed and started"
	@echo "Dashboard: http://localhost:6060"

uninstall-service:
	-launchctl unload $(LAUNCH_DIR)/$(PLIST) 2>/dev/null
	rm -f $(LAUNCH_DIR)/$(PLIST)

status:
	@$(PREFIX)/bin/$(BINARY) --status 2>/dev/null || ./$(BINARY) --status 2>/dev/null || echo "eventrelay not installed"

test:
	go test ./...

clean:
	rm -f $(BINARY)
