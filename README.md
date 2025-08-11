# VSCode Helper File Find

A dual-purpose toolkit providing:

1. Go CLI for searching files and opening them in VS Code.
2. Python MCP (Model Context Protocol) HTTP server exposing those capabilities as tools to AI / Copilot style clients.

## Features

### Go CLI
- `search` recursively searches for files by name pattern and/or text content (reports line numbers).
- `open` opens a file or directory in VS Code via the `code` command.

### MCP Server
- Serves over HTTP (default: `http://127.0.0.1:8080/mcp/`).
- Exposes two tools:
  - `search_files`
  - `open_file`
- Streamable transport using `StreamableHTTPSessionManager`.

## Repository Layout
```
├── cmd/
│   ├── root.go      # Cobra root command setup
│   ├── search.go    # Implements file search
│   └── open.go      # Implements VS Code open command
├── main.go          # CLI entrypoint
├── mcp_server.py    # MCP server bridging to Go binary
├── requirements.txt # Python dependencies for MCP server
├── go.mod / go.sum  # Go module definitions
└── Dockerfile       # Container build for MCP server + CLI
```

## Prerequisites
- Go 1.22+
- Python 3.11+
- VS Code installed with `code` CLI available in PATH (for `open` tool to work)

## Build the Go Helper Binary
This produces `./vscode-helper` (expected by `mcp_server.py`).

```bash
go build -o vscode-helper
```

## Run the MCP Server
Use a virtual environment (recommended):

```bash
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python mcp_server.py
```

Server log should show:
```
Starting streamable HTTP MCP server at http://127.0.0.1:8080/mcp/
Registered tools: search_files, open_file
```

## MCP Client Configuration (VS Code / GitHub Copilot Chat)
Create (locally, **do not commit**) a `mcp.json` (location depends on client; for VS Code user settings example):

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
```
(Expect a 200 or protocol-specific response; errors here may still indicate the endpoint is up.)

## Error Handling
- Go subprocess failures bubble up as text results beginning with `Error ...`.
- Missing binary: startup warning plus tool responses containing the exception message.

## Development Tips
- Add new tools: extend `_TOOL_DEFINITIONS` and update `_call_tool` dispatcher.
- Keep schemas strict (`additionalProperties: false`) to surface typos early.
- Use logging levels (adjust via `LOGLEVEL` env if desired): `export LOGLEVEL=DEBUG`.

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

---
Feel free to open issues or PRs for enhancements.
