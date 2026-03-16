# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Pantalk

Pantalk is a lightweight daemon (`pantalkd`) + CLI (`pantalk`) that gives AI agents a unified interface to communicate across 9+ chat platforms (Slack, Discord, Telegram, Matrix, WhatsApp, IRC, Mattermost, Twilio, Zulip, iMessage) through a single Unix domain socket protocol.

## Build & Development Commands

```bash
make build          # Build both pantalk and pantalkd binaries (CGO_ENABLED=0)
make test           # Run all tests (go test ./... -count=1)
make fmt            # Format code (go fmt ./...)
make vet            # Lint (go vet ./...)
make lint           # Combined linting
make clean          # Remove built binaries
make cross          # Cross-compile (set GOOS and GOARCH)
```

Run a single test:
```bash
go test ./internal/store/ -run TestSpecificName -count=1
```

## Architecture

Two-tier design: CLI client communicates with daemon over a Unix domain socket using a JSON protocol.

```
pantalk CLI → Unix Socket (JSON) → pantalkd → Connector Layer → Chat Platforms
```

### Key Modules

- **`cmd/pantalk/`** / **`cmd/pantalkd/`** — Entry points for CLI and daemon
- **`internal/protocol/`** — Shared JSON types for IPC (actions: ping, bots, status, send, react, history, notifications, subscribe, reload)
- **`internal/server/`** — Core daemon: manages connectors, SQLite store, subscriptions, agent dispatch
- **`internal/client/`** — CLI command dispatcher, auto-detects JSON vs TTY formatted output
- **`internal/upstream/`** — Platform connectors implementing the `Connector` interface (`Run`, `Send`, `React`, `Identity`). One file per platform.
- **`internal/config/`** — YAML config parsing with `$ENV_VAR` credential resolution, XDG path defaults
- **`internal/store/`** — SQLite persistence layer (events table with filtering/search)
- **`internal/agent/`** — Reactive agent runner with event buffering, cooldown, and `expr`-based trigger expressions
- **`internal/formatting/`** — Platform-specific markdown/mention conversion (Slack block kit, Discord, etc.)
- **`internal/ctl/`** — Admin commands (validate, reload, config modification, pairing)
- **`internal/skill/`** — OpenAPI-like capability schema generation for AI agent tools

### Key Patterns

- **Connector interface**: All platforms implement `Run`, `Send`, `React`, `Identity` — add new platforms by implementing this interface in `internal/upstream/`
- **Subscription model**: In-memory pub-sub channels per bot for real-time event streaming
- **Notification flagging**: Events marked as notifications when they're DMs, mentions, or on threads where the agent previously participated
- **Agent execution**: Events batch via configurable buffer window (default 30s) + cooldown (default 60s), then launch command with event JSON on stdin
- **Agent triggers**: Boolean expression language via `expr` package — supports `direct`, `mentions`, `notify`, `at("14:30")`, `every("15m")`, `weekday("monday")`
- **Hot reload**: `pantalk reload` restarts connectors without losing state
- **Allowed agent commands**: Restricted to `claude`, `codex`, `copilot`, `aider`, `goose`, `opencode`, `gemini` unless `--allow-exec` flag

### Config

Config file at `~/.config/pantalk/config.yaml` (override with `--config`). Defines server settings, bot credentials per platform, and agent trigger rules. See `configs/pantalk.example.yaml` for reference.

## Go Module

- Go 1.25.7, module path `github.com/pantalk/pantalk`
- Version injected at build time via `-ldflags` into `internal/version.Version`
