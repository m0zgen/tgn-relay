# tgn-relay

`tgn-relay` is a small Telegram notification gateway written in Go.

It is designed for cases where direct access to `https://api.telegram.org` is blocked or inconvenient, and for safer internal notifications where clients should not know Telegram bot tokens or chat IDs.

## Features in v0.1.0

- Send messages by configured group name: `group + text`
- Optional direct mode: `token + chat_id + text`
- `X-Relay-Key` authentication
- Optional CIDR allowlist
- Safe logs: no full URI logging by default
- JSON and form-urlencoded requests
- `/healthz` endpoint
- Caddy and systemd examples

## API

### Send by group

```bash
curl -s -X POST https://tgn-relay.example.net/api/v1/send \
  -H "X-Relay-Key: change-me-super-secret" \
  -H "Content-Type: application/json" \
  -d '{"group":"monitoring","text":"Hello from tgn-relay"}'
```

Form mode:

```bash
curl -s -X POST https://tgn-relay.example.net/api/v1/send \
  -H "X-Relay-Key: change-me-super-secret" \
  -d group=monitoring \
  -d text="Hello from Bash"
```

### Direct mode

Direct mode is disabled by default. Enable it explicitly in config:

```yaml
security:
  direct_mode_enabled: true
```

Then:

```bash
curl -s -X POST https://tgn-relay.example.net/api/v1/direct \
  -H "X-Relay-Key: change-me-super-secret" \
  -d token="123456:ABCDEF" \
  -d chat_id="-1001234567890" \
  -d text="Hello from direct mode"
```

## Config

See [`configs/config.example.yml`](configs/config.example.yml).

```yaml
listen: ":8080"

telegram:
  api_url: "https://api.telegram.org"
  timeout: "7s"

security:
  relay_keys:
    - "change-me-super-secret"
  allow_ips: []
  direct_mode_enabled: false
  max_text_bytes: 4096

groups:
  monitoring:
    token: "123456:ABCDEF"
    chat_id: "-1001234567890"
    parse_mode: "HTML"
    disable_web_page_preview: true
    disable_notification: false
```

## Build

```bash
go mod tidy
make build
```

Binary:

```bash
./bin/tgn-relay -config configs/config.example.yml
```

## Caddy

```caddyfile
tgn-relay.example.net {
    reverse_proxy 127.0.0.1:8080
}
```

## systemd

```bash
sudo useradd --system --home /var/lib/tgn-relay --shell /usr/sbin/nologin tgn-relay
sudo mkdir -p /etc/tgn-relay /var/lib/tgn-relay
sudo cp bin/tgn-relay /usr/local/bin/tgn-relay
sudo cp configs/config.example.yml /etc/tgn-relay/config.yml
sudo cp deploy/systemd/tgn-relay.service /etc/systemd/system/tgn-relay.service
sudo systemctl daemon-reload
sudo systemctl enable --now tgn-relay
```

Logs:

```bash
journalctl -u tgn-relay -f
```

## Security notes

- Prefer `/api/v1/send` over `/api/v1/direct`.
- Keep Telegram bot tokens only in `/etc/tgn-relay/config.yml`.
- Do not log request bodies.
- Do not expose this service without `X-Relay-Key` or a network allowlist.
- If you previously used a transparent proxy with `/bot<TOKEN>/sendMessage`, rotate exposed Telegram tokens.

## HTML Messages

Alert examples:

```html
<b>🚨 zBLD Alert</b>

<b>Host:</b> <code>ada.openbld.net</code>
<b>Service:</b> <code>tgn-relay</code>
<b>Status:</b> <b>OK</b>

<blockquote>
✅ Test message successfully delivered through tgn-relay.
</blockquote>

<b>Details:</b>
• Source: <code>127.0.0.1:8080</code>
• Group: <code>monitoring</code>
• Time: <code>2026-06-16 17:42:00</code>

<a href="https://openbld.net">OpenBLD.net</a>
```

Curl sender:

```bash
curl -s -X POST http://127.0.0.1:8080/api/v1/send \
  -H "X-Relay-Key: change-me-super-secret" \
  -H "Content-Type: application/json" \
  -d '{
    "group": "monitoring",
    "parse_mode": "HTML",
    "text": "<b>🚨 zBLD Alert</b>\n\n<b>Host:</b> <code>ada.openbld.net</code>\n<b>Service:</b> <code>tgn-relay</code>\n<b>Status:</b> <b>OK</b>\n\n<blockquote>✅ Test message successfully delivered through tgn-relay.</blockquote>\n\n<b>Details:</b>\n• Source: <code>127.0.0.1:8080</code>\n• Group: <code>monitoring</code>\n• Time: <code>2026-06-16 17:42:00</code>\n\n<a href=\"https://openbld.net\">OpenBLD.net</a>"
  }'
```

Monitoring message example:

```html
<b>🔥 Monitoring Event</b>

<b>Severity:</b> <code>warning</code>
<b>Node:</b> <code>srv-7.openbld.net</code>
<b>Metric:</b> <code>memory_usage</code>
<b>Value:</b> <code>87.4%</code>

<blockquote>
Memory usage is above warning threshold.
</blockquote>

<b>Action:</b>
Check process list and Prometheus dashboard.

<a href="https://openbld.net">OpenBLD Infrastructure</a>
```

Curl sender:

```bash
curl -s -X POST http://127.0.0.1:8080/api/v1/send \
  -H "X-Relay-Key: change-me-super-secret" \
  -H "Content-Type: application/json" \
  -d '{
    "group": "monitoring",
    "parse_mode": "HTML",
    "text": "<b>🔥 Monitoring Event</b>\n\n<b>Severity:</b> <code>warning</code>\n<b>Node:</b> <code>zunit-7.openbld.net</code>\n<b>Metric:</b> <code>memory_usage</code>\n<b>Value:</b> <code>87.4%</code>\n\n<blockquote>Memory usage is above warning threshold.</blockquote>\n\n<b>Action:</b>\nCheck process list and Prometheus dashboard.\n\n<a href=\"https://openbld.net\">OpenBLD Infrastructure</a>"
  }'
```

Escape characters in HTML:
- `&` → `&amp;`
- `<` → `&lt;`
- `>` → `&gt;`
- `"` → `&quot;`
- `'` → `&#39;`

Example:

```html
<code>x &lt; y</code>
```

## Credits

- OpenBLD.net team for inspiration and testing
- Go standard library and open-source ecosystem for making this possible
