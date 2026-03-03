# Security

## Threat Model

eventrelay is designed to run as a local system service or on a trusted intranet. It is **not designed for exposure to the public internet**. The threat model assumes:

- The operator controls the machine and config file
- Network access is limited to localhost or a trusted LAN
- If exposed to a network, a reverse proxy (Caddy, nginx) handles TLS and access control

## On-Device (localhost only, default)

**Default bind: `127.0.0.1`** — eventrelay only accepts connections from the local machine.

| Surface | Risk | Mitigation |
|---------|------|------------|
| POST /events | Any local process can submit events | Low risk — this is the intended use case. Add `--token` if you want to restrict which local tools can post. |
| Dashboard / Pages | Any local user can view the web UI | Low risk on a personal machine. Events and page output may contain sensitive data — be aware of what your scripts expose. |
| Page commands | Commands run as the eventrelay user | Commands are defined **only in the config file**, not via the API. An attacker would need write access to the config file, at which point they already have shell access. |
| Log file | JSONL log written with `0600` permissions | Only readable by the eventrelay user. Events may contain sensitive data in the `data` field. |
| PID file | Written to `~/.config/eventrelay/` | Standard permissions. |

## On Network (intranet deployment)

When binding to `0.0.0.0` or deploying behind a reverse proxy:

| Surface | Risk | Mitigation |
|---------|------|------------|
| POST /events | Anyone on the network can submit events | **Always set `--token`** (or `server.token` in config). SDKs pass this as `Authorization: Bearer <token>`. |
| Dashboard | Anyone on the network can view events and pages | Use Caddy/nginx basic auth to protect the UI. See `deploy/Caddyfile` for examples. |
| Page commands | Command output is visible to anyone with dashboard access | Review what your page scripts expose. Don't register commands that output secrets. |
| SSE stream | Anyone can subscribe to the event stream | The stream shows the same data as the dashboard. Protect with reverse proxy auth if needed. |

### Recommended network setup

```
Internet ←✗— (do not expose)

LAN / VPN:
  Client → Caddy (TLS + basic auth) → eventrelay (localhost:6060)
```

1. eventrelay binds to localhost (`127.0.0.1`)
2. Caddy reverse proxies with TLS and optional basic auth
3. SDKs post directly to eventrelay using the Bearer token
4. Dashboard users authenticate through Caddy

See `deploy/docker-compose.yml` and `deploy/Caddyfile` for a ready-to-use setup.

## Page Command Security

The pages system executes shell commands defined in the YAML config file. This is intentionally powerful — it's what makes eventrelay a useful portal. The security boundaries are:

1. **Config file is the trust boundary** — commands can only be registered by editing the config file. There is no API for registering commands at runtime.
2. **Output is sanitized** — all command output is HTML-escaped before rendering. The markdown renderer uses a whitelist approach: raw text is escaped first, then only safe structural elements (headings, bold, code, lists, tables) are re-introduced via pattern matching. Raw HTML in command output is displayed as text, not executed.
3. **Commands run with a 10-second timeout** — long-running or hung commands are killed.
4. **Commands inherit the server's environment** — they run as the same user with the same permissions. The `scripts_dir` setting adds a directory to PATH but does not grant additional privileges.

### What NOT to do

- Do not register commands that output secrets, tokens, or credentials
- Do not register commands that accept user input (page commands are non-interactive)
- Do not expose eventrelay to the public internet, even behind a proxy
- Do not put the config file in a world-writable location

## XSS Prevention

All user-visible content is HTML-escaped before rendering:

- **Event fields** (`source`, `action`, `data`, etc.) — escaped via `textContent` assignment
- **Page output** (text, yaml) — escaped before inserting into `<pre>` elements
- **JSON output** — escaped before syntax highlighting is applied
- **Markdown output** — each line is escaped individually, then safe structural tags are added via regex. Code blocks are escaped as a unit. No raw HTML passes through.

## Reporting Vulnerabilities

If you discover a security issue, please open a GitHub issue or contact the maintainers directly. We will respond within 3 business days.
