import os
import shlex
import asyncio
import logging
import contextlib
import inspect
from typing import List
from collections.abc import AsyncIterator

from mcp import types
from mcp.server import Server
from mcp.server.streamable_http_manager import StreamableHTTPSessionManager

from starlette.applications import Starlette
from starlette.routing import Mount
from starlette.types import Scope, Receive, Send

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("vscode-file-finder-mcp")

# MCP Server instance
mcp_server = Server("vscode-file-finder")

BIN_PATH = "./vscode-helper"

def _build_cmd(base: List[str]) -> List[str]:
    if not os.path.exists(BIN_PATH):
        raise FileNotFoundError(
            f"Missing helper binary at {BIN_PATH}. Build it first: go build -o vscode-helper"
        )
    return [BIN_PATH, *base]

async def _run_cmd(cmd: List[str]) -> str:
    logger.debug("Running command: %s", " ".join(shlex.quote(c) for c in cmd))
    proc = await asyncio.create_subprocess_exec(
        *cmd,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
    )
    stdout, stderr = await proc.communicate()
    if proc.returncode != 0:
        err = (stderr.decode() or stdout.decode() or f"exit code {proc.returncode}").strip()
        raise RuntimeError(err)
    return stdout.decode().strip()

async def _search_files_impl(
    name: str = "",
    content: str = "",
    directory: str = ".",
) -> List[types.TextContent]:
    """Internal implementation for search_files tool returning unstructured text blocks."""
    cmd = _build_cmd(["search"])
    if name:
        cmd += ["--name", name]
    if content:
        cmd += ["--content", content]
    if directory and directory != ".":
        cmd += ["--dir", directory]
    try:
        output = await _run_cmd(cmd)
        output = output or "(no matches)"
        return [types.TextContent(type="text", text=output)]
    except Exception as e:
        return [types.TextContent(type="text", text=f"Error searching: {e}")]

async def _open_file_impl(
    path: str,
    open_dir: bool = False,
) -> List[types.TextContent]:
    """Internal implementation for open_file tool returning unstructured text blocks."""
    if not path:
        return [types.TextContent(type="text", text="Error: 'path' is required")]
    cmd = _build_cmd(["open"])
    if open_dir:
        cmd.append("--dir")
    cmd.append(path)
    try:
        output = await _run_cmd(cmd)
        output = output or f"Opened: {path}"
        return [types.TextContent(type="text", text=output)]
    except Exception as e:
        return [types.TextContent(type="text", text=f"Error opening: {e}")]

# Tool definitions (JSON Schemas) used for list_tools response
_TOOL_DEFINITIONS: list[types.Tool] = [
    types.Tool(
        name="search_files",
        description="Search files by name and/or content starting at a directory.",
        inputSchema={
            "type": "object",
            "properties": {
                "name": {"type": "string", "description": "Glob or pattern for file names"},
                "content": {"type": "string", "description": "Substring / text to search inside files"},
                "directory": {"type": "string", "description": "Root directory to start search", "default": "."},
            },
            "required": [],
            "additionalProperties": False,
        },
        outputSchema=None,
    ),
    types.Tool(
        name="open_file",
        description="Open a file or directory in VS Code (uses helper binary).",
        inputSchema={
            "type": "object",
            "properties": {
                "path": {"type": "string", "description": "Path to file or directory"},
                "open_dir": {"type": "boolean", "description": "Treat path as directory", "default": False},
            },
            "required": ["path"],
            "additionalProperties": False,
        },
        outputSchema=None,
    ),
]

@mcp_server.list_tools()
async def _list_tools() -> list[types.Tool]:  # noqa: D401
    """Provide tool definitions to MCP clients."""
    return _TOOL_DEFINITIONS

@mcp_server.call_tool()
async def _call_tool(tool_name: str, arguments: dict) -> List[types.TextContent]:  # noqa: D401
    """Dispatch a tool call based on name -> implementation."""
    arguments = arguments or {}
    if tool_name == "search_files":
        return await _search_files_impl(
            name=arguments.get("name", ""),
            content=arguments.get("content", ""),
            directory=arguments.get("directory", "."),
        )
    if tool_name == "open_file":
        return await _open_file_impl(
            path=arguments.get("path", ""),
            open_dir=bool(arguments.get("open_dir", False)),
        )
    return [types.TextContent(type="text", text=f"Error: unknown tool '{tool_name}'")]

# Streamable HTTPSessionManager expects the MCP Server as its 'app'
session_manager = StreamableHTTPSessionManager(app=mcp_server)

async def mcp_endpoint(scope: Scope, receive: Receive, send: Send) -> None:
    # Delegate each HTTP request under /mcp/ to the session manager
    await session_manager.handle_request(scope, receive, send)

@contextlib.asynccontextmanager
async def lifespan(_app: Starlette) -> AsyncIterator[None]:
    # Initialize streaming task group via run() which returns an async context manager
    run_fn = getattr(session_manager, "run", None)
    if not callable(run_fn):
        logger.warning("Session manager has no run(); continuing without background tasks.")
        yield
        return

    run_cm = run_fn()
    # Detect context manager
    if hasattr(run_cm, "__aenter__") and hasattr(run_cm, "__aexit__"):
        logger.info("Starting MCP session manager run() context")
        await run_cm.__aenter__()
    else:
        # Fallback: maybe awaitable
        if inspect.isawaitable(run_cm):
            await run_cm
        else:
            logger.warning("run() returned unsupported object (%r); skipping init.", run_cm)

    try:
        logger.info("MCP session manager started")
        yield
    finally:
        if hasattr(run_cm, "__aexit__"):
            try:
                await run_cm.__aexit__(None, None, None)
            except Exception as e:
                logger.warning("Error exiting run() context: %s", e)
        else:
            # Try graceful shutdown methods if context not used
            for attr in ("close", "shutdown", "stop"):
                fn = getattr(session_manager, attr, None)
                if callable(fn):
                    try:
                        res = fn()
                        if inspect.isawaitable(res):
                            await res
                    except Exception as e:
                        logger.warning("Error during %s(): %s", attr, e)
                    break
        logger.info("MCP session manager stopped")

# Starlette app just routes /mcp/ to the session manager
starlette_app = Starlette(
    debug=False,
    routes=[
        # Mount with trailing slash to avoid 307 redirect for client config using /mcp/
        Mount("/mcp/", app=mcp_endpoint),
    ],
    lifespan=lifespan,
)

def _registered_tool_names() -> list[str]:
    return [t.name for t in _TOOL_DEFINITIONS]

def main() -> None:
    if not os.path.exists(BIN_PATH):
        logger.warning("Helper binary not found at %s. Build it: go build -o vscode-helper", BIN_PATH)
    logger.info("Starting streamable HTTP MCP server at http://127.0.0.1:8080/mcp/")
    logger.info("Registered tools: %s", ", ".join(_registered_tool_names()))
    import uvicorn
    uvicorn.run(starlette_app, host="127.0.0.1", port=8080, log_level="info")

if __name__ == "__main__":
    main()