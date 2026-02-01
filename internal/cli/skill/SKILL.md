---
name: push
description: Push notifications via Pushover - send and receive notifications. Use when the user wants to send alerts or check notification history.
---

# push - Push Notifications

Send and receive push notifications via Pushover service.

## When to use push

- User wants to send a notification
- User asks about notification history
- User wants alerts for long-running tasks
- User mentions Pushover

## Available MCP tools

| Tool | Purpose |
|------|---------|
| `mcp__push__send` | Send a notification |
| `mcp__push__history` | Get notification history |
| `mcp__push__check_status` | Check API status |

## Common patterns

### Send a notification
```
mcp__push__send(
  message="Build completed successfully!",
  title="CI/CD",
  priority=0
)
```

### Send urgent notification
```
mcp__push__send(
  message="Server is down!",
  title="Alert",
  priority=2
)
```

### Get history
```
mcp__push__history(limit=20)
```

## Priority levels

- `-2` - Lowest (no notification)
- `-1` - Low (quiet)
- `0` - Normal
- `1` - High
- `2` - Emergency (requires acknowledgment)

## CLI commands (if MCP unavailable)

```bash
push send "Message" --title "Title"
push send "Urgent!" --priority 2
push history                      # Recent notifications
push history --format markdown    # Export
```

## Configuration

Requires Pushover API credentials in `~/.config/push/config.json`

## Data location

`~/.local/share/push/push.db` (SQLite, respects XDG_DATA_HOME)
