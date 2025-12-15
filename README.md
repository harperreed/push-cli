# push

A CLI and MCP server for the [Pushover](https://pushover.net) notification service. Send and receive push notifications from the command line or integrate with AI assistants via the Model Context Protocol.

## Features

- **Send notifications** to your devices via the Pushover Message API
- **Receive messages** via the Pushover Open Client API
- **Persist messages** to a local SQLite database for history and search
- **MCP server** for AI assistant integration (Claude, etc.)
- **XDG-compliant paths** on all platforms (`~/.config/push/`, `~/.local/share/push/`)

## Prerequisites

You'll need:

1. A [Pushover](https://pushover.net) account
2. An **application token** from [pushover.net/apps](https://pushover.net/apps) (create a new application)
3. Your **user key** from [pushover.net](https://pushover.net) (shown on your dashboard)

## Installation

```bash
# From source
git clone https://github.com/harper/push.git
cd push
go build -o push .

# Or with go install
go install github.com/harper/push@latest
```

## Quick Start

### 1. Login and register a device

```bash
push login
```

You'll be prompted for:
- **App token** - from your Pushover application
- **User key** - from your Pushover dashboard
- **Email** - your Pushover account email
- **Password** - your Pushover account password
- **2FA code** - if two-factor authentication is enabled

This registers a device for receiving messages and stores credentials securely.

### 2. Send a notification

```bash
push send "Hello from the CLI!"
push send -t "Alert" -p 1 "Something important happened"
```

### 3. Check for new messages

```bash
push messages
```

### 4. View message history

```bash
push history
push history --since yesterday
push history --search "important"
```

## CLI Reference

### Global Flags

| Flag | Description |
|------|-------------|
| `--config` | Config file path (default: `~/.config/push/config.toml`) |
| `--data` | Data directory path (default: `~/.local/share/push/`) |

### Commands

#### `push login`

Authenticate with Pushover and register a device for receiving messages.

```bash
push login
push login --device-name "my-server"
```

| Flag | Description |
|------|-------------|
| `--device-name` | Device name to register (default: `push-cli`) |

#### `push logout`

Remove stored device credentials (keeps app token and user key).

```bash
push logout
```

#### `push send [message]`

Send a push notification.

```bash
push send "Simple message"
push send -t "Title" "Message with title"
push send -p 2 "Emergency priority message"
push send -u "https://example.com" "Message with link"
push send -d "iphone" "Send to specific device"
push send -s "cosmic" "Message with custom sound"
```

| Flag | Short | Description |
|------|-------|-------------|
| `--title` | `-t` | Notification title |
| `--priority` | `-p` | Priority level (-2 to 2) |
| `--url` | `-u` | Supplementary URL |
| `--url-title` | | Title for the URL |
| `--sound` | `-s` | Notification sound name |
| `--device` | `-d` | Target device name (sends to all if omitted) |

**Priority levels:**
- `-2` - Lowest (no notification)
- `-1` - Low (quiet)
- `0` - Normal (default)
- `1` - High (bypass quiet hours)
- `2` - Emergency (requires acknowledgment)

#### `push messages`

Fetch unread messages from Pushover. Messages are automatically persisted to the local database.

```bash
push messages
push messages -n 5
```

| Flag | Short | Description |
|------|-------|-------------|
| `--limit` | `-n` | Maximum messages to return (default: 10) |

#### `push history`

Query persisted message history from the local SQLite database.

```bash
push history
push history -n 50
push history --since "2025-01-01"
push history --since yesterday
push history --search "error"
```

| Flag | Short | Description |
|------|-------|-------------|
| `--limit` | `-n` | Maximum messages to return (default: 20) |
| `--since` | | Filter by date (ISO format or natural language) |
| `--search` | | Full-text search in message and title |

#### `push config`

Show current configuration.

```bash
push config           # Show config contents
push config --path    # Show config file path only
```

#### `push mcp`

Start the MCP server for AI assistant integration.

```bash
push mcp
```

The server runs on stdio and implements the Model Context Protocol.

## MCP Integration

The `push mcp` command starts a Model Context Protocol server, allowing AI assistants like Claude to send and receive Pushover notifications.

### Configuration

Add to your Claude Code MCP settings (`~/.claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "push": {
      "command": "/path/to/push",
      "args": ["mcp"]
    }
  }
}
```

### Available Tools

#### `send_notification`

Send a push notification through Pushover.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `message` | string | yes | Body of the notification |
| `title` | string | no | Notification title |
| `priority` | integer | no | Priority from -2 to 2 |
| `url` | string | no | Supplementary URL |
| `sound` | string | no | Notification sound |
| `device` | string | no | Target device name |

#### `check_messages`

Poll the Pushover Open Client API, persist new messages, and return the newest ones.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `limit` | integer | no | Maximum messages to return (default: 10) |

#### `list_history`

Query persisted message history from the local SQLite database.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `limit` | integer | no | Number of rows to return (default: 20) |
| `since` | string | no | Natural language or ISO date filter |
| `search` | string | no | Full text search over message and title |

#### `mark_read`

Delete unread messages from Pushover up to (and including) the provided ID.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `message_id` | integer | yes | Highest Pushover message ID to acknowledge |

### Available Resources

| URI | Description |
|-----|-------------|
| `push://unread` | Current unread messages (fetched live from Pushover) |
| `push://history` | Last 20 persisted messages from the local database |
| `push://status` | Credential and database health summary |

## Configuration

Configuration is stored in TOML format at `~/.config/push/config.toml`:

```toml
app_token = "your-app-token"
user_key = "your-user-key"
device_id = "device-identifier"
device_secret = "device-secret-from-login"
default_device = "push-cli"
default_priority = 0
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `XDG_CONFIG_HOME` | Override config directory (default: `~/.config`) |
| `XDG_DATA_HOME` | Override data directory (default: `~/.local/share`) |

## Data Storage

Messages are persisted to a SQLite database at `~/.local/share/push/push.db`.

The database contains two tables:
- `messages` - Received messages from Pushover
- `sent` - Log of sent notifications

## Security

- Config file is created with mode `0600` (owner read/write only)
- Credentials are stored locally, never transmitted except to Pushover's API
- Device secrets are obtained via Pushover's official Open Client API

## API Reference

Push uses two Pushover APIs:

- **[Message API](https://pushover.net/api)** - For sending notifications
- **[Open Client API](https://pushover.net/api/client)** - For receiving messages and device registration

## License

MIT
