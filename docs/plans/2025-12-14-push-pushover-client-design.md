# Push: Pushover MCP Client Design

## Overview

Push is a Go utility that integrates with the Pushover API to send and receive push notifications. It exposes these capabilities as MCP tools for AI assistants, following the patterns established by toki and chronicle.

## Core Capabilities

- **Send notifications** via Pushover Message API
- **Receive notifications** via Pushover Open Client API (polling)
- **Persist messages** to SQLite by default
- **MCP integration** for AI assistant access

## Project Structure

```
push/
├── main.go                      # Entry point → cli.Execute()
├── go.mod
├── Makefile
├── internal/
│   ├── cli/                     # Cobra commands
│   │   ├── root.go              # Root + global flags
│   │   ├── login.go             # Interactive credential setup
│   │   ├── logout.go            # Clear credentials
│   │   ├── send.go              # Send notification
│   │   ├── messages.go          # Poll/list messages
│   │   ├── history.go           # View persisted messages
│   │   ├── config.go            # Show current config
│   │   └── mcp.go               # Start MCP server
│   ├── config/                  # Config file management
│   │   └── config.go            # Load/save TOML config
│   ├── db/                      # SQLite persistence
│   │   ├── db.go                # Init, migrations
│   │   └── messages.go          # Message CRUD
│   ├── pushover/                # API client
│   │   ├── client.go            # HTTP client wrapper
│   │   ├── auth.go              # Login, device registration
│   │   ├── send.go              # Message API
│   │   └── receive.go           # Open Client API (poll)
│   └── mcp/                     # MCP server
│       ├── server.go            # Server setup
│       ├── tools.go             # send_notification, check_messages, etc.
│       └── resources.go         # Read-only views
```

## Credentials & Configuration

### Config File

Location: `~/.config/push/config.toml`

```toml
# Sending (from pushover.net/apps)
app_token = "azGDORePK8gMaC0QOYAMyEEuzJnyUi"
user_key = "uQiRzpo4DXghDmr9QzzfQu27cmVRsG"

# Receiving (from login flow)
device_id = "abc123"
device_secret = "xyzSecret789"

# Optional
default_device = ""
default_priority = 0
```

### Login Flow

`push login` performs:

1. Prompt for app token (user gets from pushover.net/apps)
2. Prompt for user key (user gets from pushover.net dashboard)
3. Prompt for email + password
4. POST to `/1/users/login.json` → get `secret`
5. Handle 2FA if HTTP 412 (prompt for code, retry)
6. POST to `/1/devices.json` with `name=push-cli`, `os=O` → get `device_id`
7. Write all to config file
8. Confirm: "✓ Logged in. Device 'push-cli' registered."

### Logout

`push logout` removes device credentials from config, optionally keeps app_token/user_key.

## Database Schema

Location: `~/.local/share/push/push.db`

```sql
-- Received messages (persisted by default)
CREATE TABLE messages (
    id INTEGER PRIMARY KEY,
    pushover_id INTEGER UNIQUE,
    umid TEXT,
    title TEXT,
    message TEXT NOT NULL,
    app TEXT,
    aid TEXT,
    icon TEXT,
    received_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    sent_at DATETIME,
    priority INTEGER DEFAULT 0,
    url TEXT,
    acked INTEGER DEFAULT 0,
    html INTEGER DEFAULT 0
);

-- Sent messages (history)
CREATE TABLE sent (
    id INTEGER PRIMARY KEY,
    message TEXT NOT NULL,
    title TEXT,
    device TEXT,
    priority INTEGER DEFAULT 0,
    sent_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    request_id TEXT
);

CREATE INDEX idx_messages_received_at ON messages(received_at);
CREATE INDEX idx_sent_sent_at ON sent(sent_at);
```

## MCP Integration

### Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `send_notification` | Send a push notification | `message` (required), `title`, `priority`, `url`, `sound` |
| `check_messages` | Poll for new messages, persist, return | `limit` (default 10) |
| `list_history` | Query persisted message history | `limit`, `since`, `search` |
| `mark_read` | Delete messages from Pushover up to ID | `message_id` |

### Resources

| Resource | Description |
|----------|-------------|
| `push://unread` | Current unread messages from Pushover |
| `push://history` | Recent persisted messages (last 20) |
| `push://status` | Connection status, credential health |

## CLI Commands

| Command | Description | Example |
|---------|-------------|---------|
| `push login` | Interactive credential setup | `push login` |
| `push logout` | Clear device credentials | `push logout` |
| `push send` | Send notification | `push send "Deploy done" -t "CI" -p 1` |
| `push messages` | Poll & display new messages | `push messages` |
| `push history` | View persisted messages | `push history --limit 20 --since yesterday` |
| `push config` | Show current configuration | `push config` |
| `push mcp` | Start MCP server (stdio) | `push mcp` |

### Send Flags

- `-t, --title` - Message title
- `-p, --priority` - Priority (-2 to 2)
- `-u, --url` - Supplementary URL
- `-s, --sound` - Notification sound
- `-d, --device` - Target specific device

### History Flags

- `-n, --limit` - Max results (default 20)
- `--since` - Filter by date (natural language)
- `--search` - Text search
- `--json` - JSON output

## Pushover API Client

### Client Structure

```go
type Client struct {
    httpClient   *http.Client
    appToken     string
    userKey      string
    deviceID     string
    deviceSecret string
}

// Auth
func (c *Client) Login(email, password string) (*LoginResponse, error)
func (c *Client) LoginWith2FA(email, password, code string) (*LoginResponse, error)
func (c *Client) RegisterDevice(secret, name string) (*DeviceResponse, error)

// Send
func (c *Client) Send(msg *Message) (*SendResponse, error)

// Receive
func (c *Client) FetchMessages() ([]Message, error)
func (c *Client) DeleteMessages(upToID int) error
```

### Data Flow: Receiving

```
MCP tool: check_messages
    ↓
pushover.FetchMessages()  →  GET /1/messages.json
    ↓
db.PersistMessages(msgs)  →  INSERT into SQLite
    ↓
pushover.DeleteMessages(highestID)  →  POST update_highest_message
    ↓
Return messages to AI
```

### Data Flow: Sending

```
MCP tool: send_notification
    ↓
pushover.Send(msg)  →  POST /1/messages.json
    ↓
db.LogSent(msg, requestID)  →  INSERT into sent table
    ↓
Return confirmation to AI
```

### Requirements

- User-Agent header: `push-cli/1.0 (darwin)` (required by Pushover)
- Max 2 concurrent HTTP requests
- Minimum 5 seconds between retries on failure

## Pushover API Reference

### Send (Message API)

- Endpoint: `POST https://api.pushover.net/1/messages.json`
- Required: `token`, `user`, `message`
- Optional: `title`, `device`, `priority`, `sound`, `timestamp`, `ttl`, `url`, `url_title`, `html`, `monospace`

### Receive (Open Client API)

- Login: `POST https://api.pushover.net/1/users/login.json` with `email`, `password`
- Register device: `POST https://api.pushover.net/1/devices.json` with `secret`, `name`, `os=O`
- Fetch messages: `GET https://api.pushover.net/1/messages.json?secret=<secret>&device_id=<id>`
- Delete messages: `POST https://api.pushover.net/1/devices/<device_id>/update_highest_message.json` with `secret`, `message`
