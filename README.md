# devup

Dev server manager for git worktrees. Wraps any dev server command, allocates a port, and makes it accessible at `<name>.devup.localhost:9909` via a [Caddy](https://caddyserver.com) reverse proxy.

## Install

```bash
go install github.com/semistrict/devup@latest
```

Requires [Caddy](https://caddyserver.com/docs/install) in your PATH.

## Usage

```bash
# Run a dev server
devup vite dev

# HTTPS mode (run `caddy trust` once first)
devup -s vite dev
```

devup will:
- Allocate a random free port and set `PORT` env var
- Start the command as a subprocess
- Start Caddy (if not already running) and register a reverse proxy route
- Show a TUI with live output, the proxy URL, and keyboard shortcuts

### Keyboard shortcuts

- **Shift-R** — restart the dev server
- **Ctrl-C** — stop and exit

### Subcommands

```bash
devup status  # List all running dev servers
```

## How it works

- **Project detection**: Finds project name from `package.json`, `Cargo.toml`, `pyproject.toml`, `go.mod`, or the git directory name
- **Worktree support**: Detects git worktrees and prefixes the hostname (e.g. `feature-myapp.devup.localhost`)
- **Reverse proxy**: Uses Caddy's admin API to dynamically register/deregister routes
- **Restart detection**: Running devup twice in the same project with the same args kills the previous instance
- **Logs**: Output is logged to `.devup/log/<command>.log` in the project directory

## Configuration

Caddy state is stored in `~/.devup/`. Project logs are stored in `<project>/.devup/log/`.
