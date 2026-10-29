# LiteHarness

Tiny, cross-platform AI coding harness styled after Claude Code. Written in Go — no runtime dependencies, statically linked Linux binaries, single-file macOS binaries.

## Features

- **Interactive REPL** — `>` prompt with streaming responses
- **Tool use** — full Claude Code-style tool set: read, write, edit, grep, glob, ls, run_command, todo_write
- **Slash commands** — `/help`, `/exit`, `/clear`, `/model`, `/provider`, `/system`, `/tools`, `/save`
- **Multiple providers** — OpenAI-compatible (Ollama, OpenRouter, NVIDIA NIM, generic) and Gemini
- **Streaming** — tokens print as they arrive; no buffering the full response
- **32-bit + 64-bit** — cross-compile from one machine to any target
- **Tiny footprint** — ~10 MB binary, no `libcurl`, no Node/Python/Rust toolchain

## Build

All targets from the same source, no cross-toolchain needed:

```bash
# 32-bit x86 Linux
GOOS=linux GOARCH=386 go build -o liteharness-linux-386 .

# 64-bit x86 Linux (statically linked)
GOOS=linux GOARCH=amd64 go build -o liteharness-linux-amd64 .

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -o liteharness-darwin-amd64 .

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o liteharness-darwin-arm64 .
```

Copy the binary to your machine and run it — no installation required on Linux.

## Configuration

Configuration is read from (in priority order): CLI flags > environment variables > `~/.liteharnessrc`.

### Environment variables

| Variable | Description | Default |
|---|---|---|
| `LITEHARNESS_PROVIDER` | `openai` or `gemini` | `openai` |
| `LITEHARNESS_BASE_URL` | API base URL (OpenAI-compatible) | `https://api.openai.com/v1` |
| `LITEHARNESS_MODEL` | Model name | `gpt-3.5-turbo` |
| `LITEHARNESS_API_KEY` | API key | *(required)* |
| `LITEHARNESS_SYSTEM_PROMPT` | System prompt | `You are a helpful coding assistant.` |

### Config file (`~/.liteharnessrc`)

Simple `key=value` format:

```
provider=openai
base_url=https://api.openai.com/v1
model=gpt-3.5-turbo
api_key=sk-...
system_prompt=You are a helpful coding assistant.
```

Use `/save` inside LiteHarness to write the current config to this file.

## Usage

```bash
# OpenAI-compatible (Ollama, OpenRouter, NVIDIA NIM, etc.)
./liteharness \
  --provider openai \
  --base-url http://localhost:11434/v1 \
  --model llama3.1 \
  --api-key ollama

# Gemini
./liteharness \
  --provider gemini \
  --model gemini-1.5-flash \
  --api-key AIza...

# CLI flags override env vars and config file
./liteharness --model gpt-4o --system-prompt "You are a senior engineer."
```

## Slash commands

| Command | Description |
|---|---|
| `/help` | Show help |
| `/exit` / `/quit` | Exit LiteHarness |
| `/clear` | Clear conversation history |
| `/model [name]` | Get or set the model |
| `/provider [name]` | Get or set the provider (`openai`, `gemini`) |
| `/system [text]` | Get or set the system prompt |
| `/tools` | List available tools with schemas |
| `/save` | Save config to `~/.liteharnessrc` |

## Tools

LiteHarness exposes these tools to the model:

| Tool | Description |
|---|---|
| `read_file` | Read a file |
| `write_file` | Write or overwrite a file |
| `edit_file` | Replace a specific string in a file |
| `ls` | List a directory |
| `grep` | Search file contents with regex |
| `glob` | Find files matching a pattern |
| `run_command` | Run a shell command |
| `todo_write` | Manage a todo list |

The model requests tool calls during the conversation; LiteHarness executes them and streams results back until the model gives a final answer.

## Example session

```
> /provider openai
Provider set to: openai

> Read the file README.md and summarize it.
[tool] read_file({"path": "README.md"})
[result] # LiteHarness
...
LiteHarness is a tiny, cross-platform AI coding harness...
> 
```

## License

MIT
