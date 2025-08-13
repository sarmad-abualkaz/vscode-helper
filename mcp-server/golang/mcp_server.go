package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Implementation metadata for the MCP server
var impl = &mcp.Implementation{Name: "vscode-file-finder-go", Version: "0.1.0"}

// SearchFilesParams defines inputs for the search_files tool
// jsonschema tags are used by the SDK to derive the input schema
// keeping names aligned with the Python server version.
type SearchFilesParams struct {
	Name      string `json:"name" jsonschema:"Glob or pattern for file names"`
	Content   string `json:"content" jsonschema:"Substring / text to search inside files"`
	Directory string `json:"directory" jsonschema:"Root directory to start search (default: ".")"`
}

// OpenFileParams defines inputs for the open_file tool
type OpenFileParams struct {
	Path    string `json:"path" jsonschema:"Path to file or directory"`
	OpenDir bool   `json:"open_dir" jsonschema:"Treat path as directory"`
}

// resolve helper binary path: VS_CODE_HELPER_BIN or ./vscode-helper or LookPath("vscode-helper")
func helperBin() (string, error) {
	if env := strings.TrimSpace(os.Getenv("VS_CODE_HELPER_BIN")); env != "" {
		return env, nil
	}
	// Prefer local project binary like Python server
	local := "./vscode-helper"
	if st, err := os.Stat(local); err == nil && !st.IsDir() {
		return local, nil
	}
	if lp, err := exec.LookPath("vscode-helper"); err == nil {
		return lp, nil
	}
	return "", fmt.Errorf("vscode-helper binary not found; build it with: go build -o vscode-helper")
}

func runHelper(ctx context.Context, args ...string) (string, error) {
	bin, err := helperBin()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf(msg)
	}
	out := strings.TrimSpace(stdout.String())
	return out, nil
}

// searchFiles implements the ToolHandlerFor signature by delegating to the helper binary.
func searchFiles(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[SearchFilesParams]) (*mcp.CallToolResultFor[any], error) {
	p := params.Arguments
	var args []string
	args = append(args, "search")
	if strings.TrimSpace(p.Name) != "" {
		args = append(args, "--name", p.Name)
	}
	if strings.TrimSpace(p.Content) != "" {
		args = append(args, "--content", p.Content)
	}
	if dir := strings.TrimSpace(p.Directory); dir != "" && dir != "." {
		args = append(args, "--dir", dir)
	}
	out, err := runHelper(ctx, args...)
	if err != nil {
		return textResult("Error searching: " + err.Error()), nil
	}
	if out == "" {
		out = "(no matches)"
	}
	return textResult(out), nil
}

// openFile delegates to helper binary 'open' command exactly like Python server
func openFile(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[OpenFileParams]) (*mcp.CallToolResultFor[any], error) {
	p := params.Arguments
	if strings.TrimSpace(p.Path) == "" {
		return textResult("Error: 'path' is required"), nil
	}
	var args []string
	args = append(args, "open")
	if p.OpenDir {
		args = append(args, "--dir")
	}
	// Pass the path as provided; the helper will resolve/validate and call 'code'
	args = append(args, p.Path)
	out, err := runHelper(ctx, args...)
	if err != nil {
		return textResult("Error opening: " + err.Error()), nil
	}
	if out == "" {
		// CLI prints confirmation; but ensure some response
		abs, _ := filepath.Abs(p.Path)
		out = "Opened: " + abs
	}
	return textResult(out), nil
}

func textResult(s string) *mcp.CallToolResultFor[any] {
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: s}},
	}
}

// createServer constructs the MCP server and registers tools.
func createServer() *mcp.Server {
	server := mcp.NewServer(impl, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "search_files", Description: "Search files by name and/or content starting at a directory."}, searchFiles)
	mcp.AddTool(server, &mcp.Tool{Name: "open_file", Description: "Open a file or directory in VS Code (uses 'code' CLI)."}, openFile)
	return server
}

func main() {
	// Flags to choose transport and address/path for HTTP mode
	httpMode := flag.Bool("http", false, "Serve over Streamable HTTP instead of stdio")
	addr := flag.String("addr", ":8081", "HTTP listen address (host:port)")
	mcpPath := flag.String("path", "/mcp", "HTTP path to mount the MCP handler")
	flag.Parse()

	if !*httpMode {
		// Default: stdio transport
		server := createServer()
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}
		return
	}

	// HTTP Streamable transport using StreamableHTTPHandler
	server := createServer()
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return server }, nil)

	mux := http.NewServeMux()
	// Health endpoints
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("healthy"))
	})
	// Mount handler at both /path and /path/ to avoid redirects/edge cases
	p := *mcpPath
	if p == "" {
		p = "/mcp"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// Ensure a variant without trailing slash
	mux.Handle(p, handler)
	if !strings.HasSuffix(p, "/") {
		mux.Handle(p+"/", handler)
	}

	srv := &http.Server{Addr: *addr, Handler: mux}
	go func() {
		log.Printf("MCP streamable HTTP server listening at http://%s%s\n", strings.TrimPrefix(*addr, ":"), p)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
