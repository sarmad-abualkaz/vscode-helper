# VS Code Helper File Find - AI Agent Instructions

## Project Overview
This is a dual-purpose tool that provides both a CLI interface and an MCP server for file operations in VS Code environments. The project combines file search capabilities with VS Code integration.

## Core Components

### CLI Commands (`cmd/`)
- **Search Command** (`cmd/search.go`): Implements file search by name and content
  - Uses `filepath.Walk` for recursive search
  - Supports case-insensitive filename pattern matching
  - Provides content search with line number reporting

- **Open Command** (`cmd/open.go`): Handles VS Code file/directory opening
  - Uses `code` CLI command for VS Code integration
  - Supports directory mode via `--dir` flag
  - Converts relative paths to absolute

### MCP Server (`internal/mcp/`)
- HTTP server exposing CLI functionality via REST API
- Endpoints:
  - `/search` - File search operations
  - `/open` - VS Code integration
  - `/health` - Server health check

## Key Patterns

### Error Handling
```go
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
```
Use formatted error messages with descriptive prefixes for CLI feedback.

### Path Handling
- Always convert to absolute paths before operations
- Use `filepath` package functions for cross-platform compatibility
- Validate paths exist before operations

## Development Workflow

### Building
```bash
go build -o vscode-helper
```

### Running MCP Server
```bash
# Local development
./vscode-helper serve --port 8080

# Docker
docker build -t vscode-helper-mcp .
docker run -p 8080:8080 vscode-helper-mcp
```

### Testing API Endpoints
```bash
curl -X POST http://localhost:8080/search \
  -H "Content-Type: application/json" \
  -d '{"name": "*.go", "content": "", "dir": "."}'
```

## Integration Points
1. VS Code CLI Integration
   - Requires `code` command in PATH
   - Uses standard VS Code CLI arguments

2. MCP Protocol Integration
   - REST API with JSON payloads
   - Standard HTTP status codes for error handling
   - Structured response format with `success`, `message`, and `data` fields

## Common Tasks
- Search files: `./vscode-helper search --name "*.go" --content "TODO"`
- Open in VS Code: `./vscode-helper open path/to/file`
- Open directory: `./vscode-helper open --dir path/to/dir`
