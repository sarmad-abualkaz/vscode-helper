FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Install VS Code CLI
RUN curl -fsSL https://code.visualstudio.com/sha/download?build=stable&os=cli-alpine-x64 | tar -xz -C /usr/local/bin

# Copy Python requirements and install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy Go binary (build it first on host)
COPY vscode-helper /app/vscode-helper
RUN chmod +x /app/vscode-helper

# Copy MCP server
COPY mcp_server.py /app/mcp_server.py

# Expose port for HTTP MCP server
EXPOSE 8080

# Set environment variables
ENV PYTHONUNBUFFERED=1
ENV PATH="/app:${PATH}"

# Run the MCP server
CMD ["python", "mcp_server.py"]