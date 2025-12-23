# AI CLI Server

A Go-based server that wraps GitHub Copilot CLI and Cursor CLI, exposing them via authenticated REST API with per-client usage tracking, rate limiting, and cost management.

## Why?

An experiment born from "why not?" — if you already have a GitHub Copilot or Cursor subscription, why not expose it for your home lab tools? This server lets you leverage existing AI subscriptions through a simple REST API, making it easy to integrate AI capabilities into scripts, automation, and other tools that support API key authentication.

## Features

- 🔐 **API Key Authentication** - Secure per-client API keys with SHA-256 hashing
- 🚦 **Rate Limiting** - Per-client rate limits with token bucket implementation
- 📊 **Usage Tracking** - Comprehensive logging with token counts and cost calculations
- 🔌 **Modular CLI Providers** - Easily add new AI CLI tools
- 💾 **SQLite Database** - Lightweight persistent storage for all data
- 🎯 **Model Access Control** - Define which models each client can access
- 📈 **Usage Analytics** - Query usage stats by provider, model, and time range

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      ./bin/server                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  HTTP Server          TUI Manager         Automation        │
│  (default)            (--manage)          (--add/--list/    │
│                                            --delete/        │
│  ┌──────────────┐    ┌──────────────┐      --models)        │
│  │  REST API    │    │  Interactive │    ┌──────────────┐   │
│  │  /v1/chat/*  │    │   Forms      │    │  JSON I/O    │   │
│  │  /v1/usage/* │    │  (huh lib)   │    │  Scripting   │   │
│  └──────────────┘    └──────────────┘    └──────────────┘   │
│         │                   │                   │           │
│         └───────────────────┼───────────────────┘           │
│                             │                               │
│                    ┌────────▼────────┐                      │
│                    │    Database     │                      │
│                    │    (SQLite)     │                      │
│                    │                 │                      │
│                    │  • Clients      │                      │
│                    │  • Usage Logs   │                      │
│                    └────────┬────────┘                      │
│                             │                               │
│                    ┌────────▼────────┐                      │
│                    │     Agents      │                      │
│                    │   (Providers)   │                      │
│                    └────────┬────────┘                      │
│                             │                               │
│              ┌──────────────┴──────────────┐                │
│              │                             │                │
│       ┌──────▼──────┐               ┌──────▼──────┐         │
│       │   Copilot   │               │   Cursor    │         │
│       │    CLI      │               │    CLI      │         │
│       │             │               │             │         │
│       │ Models from │               │ Models from │         │
│       │ `copilot -h`│               │`cursor -h`  │         │
│       └─────────────┘               └─────────────┘         │
└─────────────────────────────────────────────────────────────┘

Client Model:
┌─────────────────────────────────────┐
│  Client                             │
│  ├── name                           │
│  ├── api_key_hash                   │
│  ├── provider (copilot OR cursor)   │
│  ├── allowed_models                 │
│  ├── default_model                  │
│  └── rate_limit_per_minute          │
└─────────────────────────────────────┘
```

## Prerequisites

- Go 1.21 or later
- GitHub Copilot CLI (`copilot`) installed and authenticated
- Cursor CLI (`cursor-agent`) installed and authenticated (optional)

### Installing CLI Tools

**GitHub Copilot CLI:**
```bash
npm install -g @github/copilot
# or
brew install copilot-cli
# or
curl -fsSL https://gh.io/copilot-install | bash
```

**Cursor CLI:**
```bash
curl https://cursor.com/install -fsS | bash
```

## Installation

1. Clone the repository:

```bash
git clone https://github.com/andrew/ai-cli-server.git
cd ai-cli-server
```

2. Install dependencies:

```bash
go mod download
```

3. Set up environment variables:

```bash
export COPILOT_GITHUB_TOKEN="your-github-token"
export CURSOR_API_KEY="your-cursor-api-key"
```

or use `login` commands of respective CLIs.

4. Build the server:

```bash
go build -o bin/server ./cmd/server
```

## Configuration

Edit `configs/config.yaml`:

```yaml
server:
  host: "localhost"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

database:
  path: "./data/ai-cli-server.db"

cli:
  copilot:
    binary_path: "copilot"
    timeout: 120s
  cursor:
    binary_path: "cursor-agent"
    timeout: 120s
```

## Usage

### Running Modes

The server binary supports two modes:

**1. Server Mode (default)** - Run the HTTP API server:

```bash
./bin/server
```

**2. Client Management Mode** - Interactive CLI for managing clients:

```bash
./bin/server --manage
```

### Client Management CLI

The interactive CLI allows you to manage clients without needing API calls:

```bash
$ ./bin/server --manage
```

**Features:**

- Interactive prompts for all configuration
- Provider auto-detection (checks which CLI tools are installed)
- Model selection from available options fetched from the CLI tools
- Safe client deletion with confirmation
- Delete client and all associated history

### Start the Server

```bash
./bin/server
```

The server will start on `http://localhost:8080`.

### Send Chat Completion Request

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer aics_<your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "copilot",
    "model": "claude-sonnet-4.5",
    "messages": [
      {"role": "user", "content": "What is the capital of France?"}
    ]
  }'
```

Response:

```json
{
  "id": "chatcmpl-1",
  "provider": "copilot",
  "model": "claude-sonnet-4.5",
  "content": "The capital of France is Paris.",
  "prompt_tokens": 8,
  "completion_tokens": 7,
  "total_tokens": 15,
  "cost": 0.00045
}
```

### Query Usage Logs

```bash
curl -X GET "http://localhost:8080/v1/usage?limit=10" \
  -H "Authorization: Bearer aics_<your-api-key>"
```

### Get Usage Statistics

```bash
curl -X GET "http://localhost:8080/v1/usage/stats" \
  -H "Authorization: Bearer aics_<your-api-key>"
```

Response:

```json
{
  "total_requests": 42,
  "total_tokens": 15840,
  "total_cost": 0.47,
  "by_provider": {
    "copilot": 30,
    "cursor": 12
  },
  "by_model": {
    "claude-sonnet-4.5": 25,
    "gpt-4o": 17
  }
}
```

## API Reference

### Public Endpoints

#### `POST /v1/chat/completions`

Execute a chat completion request.

**Headers:**

- `Authorization: Bearer <api_key>` (required)

**Request Body:**

```json
{
  "provider": "copilot",  // or "cursor" (optional, auto-detected)
  "model": "claude-sonnet-4.5",
  "messages": [
    {"role": "user", "content": "Your prompt"}
  ],
  "force": false,  // Skip confirmations
  "allow_tools": ["shell(git)"],  // Copilot only
  "deny_tools": ["shell(rm)"]  // Copilot only
}
```

#### `GET /v1/usage`

Retrieve usage logs.

**Query Parameters:**
- `limit` (default: 100)
- `offset` (default: 0)
- `start_time` (RFC3339 format)
- `end_time` (RFC3339 format)

#### `GET /v1/usage/stats`

Get aggregated usage statistics.

**Query Parameters:**

- `start_time` (RFC3339 format)
- `end_time` (RFC3339 format)

## Client Management

Clients are managed via the interactive CLI (not API endpoints):

```bash
./bin/server --manage
```

**Available actions:**
- **Add new client** - Create a client with API key generation
- **List clients** - View all registered clients
- **Delete client** - Remove client and all their usage history

**Note:** Admin API endpoints have been removed in favor of the safer, interactive CLI approach.

## Development

### Build for Production

```bash
go build -ldflags="-s -w" -o bin/server ./cmd/server
```

### Database Schema

The SQLite database includes the following tables:
- `clients` - API key management
- `usage_logs` - Request tracking
- `rate_limit_buckets` - Rate limiting state

## Security Considerations

- API keys are hashed with SHA-256 before storage
- Environment variables should be used for sensitive credentials
- Admin endpoints should be protected with additional authentication in production
- Consider using HTTPS in production
- Rate limiting prevents abuse

## Adding New CLI Providers

1. Create a new package in `internal/cli/<provider>/`
2. Implement the `cli.Provider` interface
3. Register the provider in `cmd/server/main.go`
4. Update configuration in `configs/config.yaml`

## License

See [LICENSE](LICENSE) file.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.
