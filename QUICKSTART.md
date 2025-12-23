# Quick Start Guide

## 1. Build the Server

```bash
go build -o bin/server ./cmd/server
```

## 2. Set Environment Variables

```bash
export COPILOT_GITHUB_TOKEN="your-github-token"
export CURSOR_API_KEY="your-cursor-key"  # Optional
```

or use `login` commands of respective CLIs.

## 3. Create Your First Client

```bash
./bin/server --manage
```

Follow the interactive prompts:
- Select **"1. Add new client"**
- Enter a client name (e.g., "my-app")
- Choose which providers to enable (copilot/cursor)
- Select which models to allow
- Set a rate limit (e.g., 60 requests per minute)

**Save the generated API key!** It's only shown once.

## 4. Start the Server

```bash
./bin/server
```

Server runs on `http://localhost:8080`

## 5. Test Your Setup

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer aics_YOUR_KEY_HERE" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "copilot",
    "model": "claude-sonnet-4.5",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

## 6. Monitor Usage

```bash
curl -X GET "http://localhost:8080/v1/usage/stats" \
  -H "Authorization: Bearer aics_YOUR_KEY_HERE"
```

## Common Commands

**Start server (default):**

```bash
./bin/server
```

**Manage clients:**

```bash
./bin/server --manage
```

**Run automated test:**

```bash
./test.sh
```

## Troubleshooting

**No providers available:**

- Make sure GitHub Copilot CLI is installed: `which copilot`
- Make sure Cursor CLI is installed: `which cursor-agent`
- Ensure environment tokens are set correctly

**Authentication failed:**

- Check your API key starts with "aics_"
- Verify the client wasn't deleted
- Make sure the Bearer token is included in the Authorization header

**Rate limit exceeded:**

- Wait for the rate limit window to reset (1 minute)
- Increase rate limit via client management CLI
