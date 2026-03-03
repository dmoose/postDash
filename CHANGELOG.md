# Changelog

All notable changes to eventrelay are documented in this file.

## Unreleased

### Added
- **Pages system** — config-registered shell commands rendered as dashboard tabs (text, JSON, YAML, markdown)
- Built-in **Status page** with server info, event stats, and config summary
- **Bundled scripts**: `er-system`, `er-ports`, `er-services`, `er-brew`, `er-example`
- `scripts_dir` config for PATH management (solves launchd minimal PATH)
- Markdown renderer with tables, blockquotes, ordered lists, horizontal rules, code blocks
- **Docker + Caddy deployment** with docker-compose.yml and Caddyfile (TLS + basic auth)
- Server settings in YAML config (`server.port`, `bind`, `token`, `buffer`, `log`, `scripts_dir`)
- `eventrelay send` CLI command for emitting events from scripts and the terminal
- `--version` flag and `version` subcommand with build-time version embedding
- `POST /events/batch` endpoint for submitting multiple events in a single request
- `GET /healthz` endpoint returning `{"ok":true,"version":"..."}`
- CORS preflight (OPTIONS) handling for cross-origin browser clients
- Graceful shutdown on SIGINT/SIGTERM — drains connections, flushes logs, closes DB
- `Cache-Control: no-cache` on embedded static files
- SECURITY.md with threat model for local and network deployments
- Channel filter input in web dashboard
- Inline event data display with expandable detail view
- `make upgrade` and `make restart-service` targets

### Fixed
- Server now validates that `source` is present on `POST /events` and `/events/batch`
- XSS prevention in JSON syntax highlighter (escape before highlight)
- Log file permissions changed from 0644 to 0600

## v0.1.0 — Initial Release

### Added
- Real-time event streaming server with ring buffer and SSE fan-out
- Web dashboard with live event feed, stats, sparkline, and channel tabs
- Terminal UI (TUI) dashboard with filtering, pause, and color-coded levels
- REST API: POST events, SSE stream, recent events, stats, rate history, channels
- Notification system with Slack, Discord, and generic webhook targets
- Database archival via SQLite (notification rule target)
- Go SDK with fire-and-forget async sending, `Timed()` helper, and slog integration
- Python SDK with thread-safe async sending and `timed()` context manager
- TypeScript SDK with promise-based async sending and `timed()` helper
- Bearer token authentication for network deployments
- JSONL event logging to file
- PID file management and `--status` command
- macOS launchd service integration
- Configuration via YAML with match rules (AND logic)
