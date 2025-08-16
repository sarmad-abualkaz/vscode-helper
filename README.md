# VSCode Helper File Find

A dual-purpose toolkit providing:

1) Go CLI for searching files and opening them in VS Code.
2) MCP servers (Python and Go) exposing those capabilities to AI/Copilot clients.

## Features

### Go CLI
- `search` recursively searches for files by name pattern and/or text content (reports line numbers).
- `open` opens a file or directory in VS Code via the `code` command.

### MCP Servers
- Tools (both servers):
  - `search_files(name?, content?, directory?)`
  - `open_file(path, open_dir?)`

- Python HTTP server
  - Streamable HTTP via `StreamableHTTPSessionManager`
  - Default endpoint: `http://127.0.0.1:8080/mcp/`

- Go server
  - Reuses the same helper binary; handlers shell out to `vscode-helper`
  - Transports:
    - stdio (default)
    - Streamable HTTP via `StreamableHTTPHandler` (opt-in with `--http`)

## Repository Layout
```
├── cmd/
│   ├── root.go                 # Cobra root command setup
│   ├── search.go               # Implements file search
│   ├── open.go                 # Implements VS Code open command
│   └── mcp-go-server/main.go   # Go MCP server (stdio or HTTP)
├── main.go                     # CLI entrypoint for vscode-helper
├── mcp-server/
│   ├── python3/mcp_server.py   # Python HTTP MCP server (streamable)
│   └── mcp-server/golang/mcp_server.go   # Go MCP server (stdio or HTTP)
├── mcp-server/
│   ├── python3/mcp_server.py   # Python HTTP MCP server (streamable)
│   └── golang/mcp_server.go    # Go MCP server (stdio or HTTP)
├── requirements.txt            # Python dependencies for MCP server
├── go.mod / go.sum             # Go module definitions
└── Dockerfile                  # Container build for MCP server + CLI
```

## Prerequisites
- Go 1.22+
- Python 3.11+
- VS Code installed with `code` CLI available in PATH (for `open` tool to work)

## Build the Go Helper Binary
This produces `./vscode-helper` (used by both MCP servers).

```bash
go build -o vscode-helper
```

## Run the MCP Servers

### Python HTTP server (streamable HTTP)
Use a virtual environment (recommended):

```bash
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python mcp-server/python3/mcp_server.py
```

Server log should show:
```
Starting streamable HTTP MCP server at http://127.0.0.1:8080/mcp/
Registered tools: search_files, open_file
```

### Go MCP server (stdio or HTTP)
Build and run:

```bash
go build -o mcp-go-server mcp-server/golang/mcp_server.go

# stdio transport (default)
./mcp-go-server

# HTTP transport (streamable HTTP)
./mcp-go-server --http --addr :8081 --path /mcp
# Endpoint: http://127.0.0.1:8081/mcp
```

Notes:
- The Go MCP server shells out to `vscode-helper`. You can override its path via `VS_CODE_HELPER_BIN`.
- The `open_file` tool requires the `code` CLI in PATH.

## MCP Client Configuration (VS Code / GitHub Copilot Chat)
Create (locally, do not commit) a `mcp.json` (VS Code user settings example). Use one of the following:

HTTP (Python or Go servers):
```jsonc
{
  "servers": {
    "vscode-file-finder": {
      "type": "http",
      "url": "http://localhost:8080/mcp/",
      "timeout": 30000
    }
  }
}
```

stdio (Go server):
```jsonc
{
  "servers": {
    "vscode-file-finder": {
      "type": "stdio",
      "command": "/absolute/path/to/mcp_server",
      "args": [],
      "timeout": 30000
    }
  }
}
```

Reload VS Code; the client should discover two tools.

## Using the Tools in Chat
Examples (natural language or structured):
- `Use search_files to find *.go mentioning TODO` 
- `/tool vscode-file-finder search_files {"name":"*.py","content":"async"}`
- `/tool vscode-file-finder open_file {"path":"mcp_server.py"}`
- Open a directory: `/tool vscode-file-finder open_file {"path":"cmd","open_dir":true}`

## REST Testing (Basic Reachability)
Although the MCP endpoint expects protocol messages, a plain POST can confirm reachability:
```bash
curl -X POST http://127.0.0.1:8080/mcp/ -d '{}' -H 'Content-Type: application/json'
# or if using the Go HTTP server on 8081
curl -X POST http://127.0.0.1:8081/mcp -d '{}' -H 'Content-Type: application/json'
```
(Expect a 200 or protocol-specific response; errors here may still indicate the endpoint is up.)

## Error Handling
- Go subprocess failures bubble up as text results beginning with `Error ...`.
- Missing binary: startup warning plus tool responses containing the exception message.

## Development Tips
- Add new tools: extend `_TOOL_DEFINITIONS` and update `_call_tool` dispatcher.
- Keep schemas strict (`additionalProperties: false`) to surface typos early.
- Use logging levels (adjust via `LOGLEVEL` env if desired): `export LOGLEVEL=DEBUG`.
- Go MCP handlers live in `cmd/mcp-go-server/main.go` and delegate to `vscode-helper`.
- You can point the Go server at a specific helper path via `VS_CODE_HELPER_BIN`.

## Docker
Build and run:
```bash
docker build -t vscode-helper-mcp .
docker run -p 8080:8080 vscode-helper-mcp
```

## Roadmap Ideas
- Add structured JSON output schema for search results.
- Pagination / streaming for large search outputs.
- WebSocket or SSE endpoint for push updates.
- Authentication layer (API key / token) for multi-user setups.

## License
MIT (add a LICENSE file if distributing publicly).

## Troubleshooting
| Symptom | Cause | Fix |
|---------|-------|-----|
| `Discovered 0 tools` | list_tools not called yet or wrong URL | Ensure URL ends with `/mcp/`, reload client |
| `Error: 'path' is required` | Missing required param for open_file | Provide `path` field |
| Exit code errors | Go binary not built or crashed | Rebuild: `go build -o vscode-helper` |
| 307 redirect then 500 | URL missing trailing slash | Use `/mcp/` in config |
| Go server returns "vscode-helper not found" | Helper binary missing | Build helper: `go build -o vscode-helper` or set `VS_CODE_HELPER_BIN` |

---
Feel free to open issues or PRs for enhancements.
