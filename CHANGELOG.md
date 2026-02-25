# Changelog

All notable changes to eventrelay are documented in this file.

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
