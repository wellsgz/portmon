# eBPF Port Traffic Monitor

[![CI](https://github.com/wellsgz/portmon/actions/workflows/ci.yml/badge.svg)](https://github.com/wellsgz/portmon/actions/workflows/ci.yml)
[![Release](https://github.com/wellsgz/portmon/actions/workflows/release.yml/badge.svg)](https://github.com/wellsgz/portmon/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A lightweight, real-time network traffic monitoring tool using eBPF kprobes to monitor bidirectional TCP traffic on specified ports. Features persistent historical data storage and both CLI and TUI interfaces.

## Features

- 🚀 **eBPF-powered** - Minimal overhead using kernel-level packet tracing
- 📊 **Realtime + Historical** - Live stats and SQLite-backed historical data
- 🖥️ **TUI Dashboard** - Interactive terminal UI with date range selection
- 💾 **Billing Cycle Support** - Custom date ranges for usage tracking
- 🔧 **Systemd Ready** - Includes service file for production deployment

## Requirements

- **Linux kernel 5.4+** with BTF support
- **Go 1.21+**
- **clang/llvm** and **libbpf-dev** (for eBPF compilation)
- Root privileges (for eBPF loading)

## Installation

### From Release

```bash
# Download latest release
curl -LO https://github.com/wellsgz/portmon/releases/latest/download/portmon-linux-amd64.tar.gz
tar -xzf portmon-linux-amd64.tar.gz
sudo mv portmond portmon /usr/local/bin/
```

### From Source

```bash
git clone https://github.com/wellsgz/portmon.git
cd portmon

# Install dependencies
make deps

# Generate vmlinux.h and build
make vmlinux generate build
```

## Quick Start

```bash
# Start daemon (requires root)
sudo ./bin/portmond --port 5000 --port 8080

# Launch TUI (another terminal)
./bin/portmon tui

# Or use CLI
./bin/portmon stats --port 5000
./bin/portmon stats --port 5000 --today
./bin/portmon stats --port 5000 --cycle-day 15  # Billing cycle
./bin/portmon status
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Daemon (portmond)                        │
│  ┌──────────┐  ┌────────────┐  ┌────────────┐               │
│  │  eBPF    │─▶│ Aggregator │─▶│ IPC Server │               │
│  │ Collector│  └─────┬──────┘  └──────┬─────┘               │
│  └──────────┘        │                │                     │
│                      ▼                │                     │
│               ┌──────────┐            │                     │
│               │ SQLite   │            │                     │
│               └──────────┘            │                     │
└───────────────────────────────────────┼─────────────────────┘
                                        │ Unix Socket
                                        ▼
                              ┌─────────────────┐
                              │ CLI/TUI Client  │
                              │   (portmon)     │
                              └─────────────────┘
```

## Configuration

### Config File (Recommended)

Create `/etc/portmon/portmon.yaml`:

```yaml
ports:
  - 5000
  - 8080

data_dir: /var/lib/portmon
socket: /run/portmon/portmon.sock
retention_days: 90
log_level: info
```

Then run: `sudo portmond` or `sudo portmond -c /path/to/config.yaml`

### CLI Options

CLI flags override config file values:

```bash
portmond \
  --config /etc/portmon/portmon.yaml \
  --port 5000 \               # Ports to monitor (repeatable)
  --data-dir ~/.portmon \     # Data directory
  --retention-days 180 \      # Data retention (1-365 days)
  --socket ~/.portmon/portmon.sock \
  --log-level info            # debug, info, warn, error

# CLI options
portmon stats --port 5000 --today       # Today's stats
portmon stats --port 5000 --cycle-day 15  # Billing cycle (15th-14th)
portmon stats --port 5000 --json        # JSON output
```

## TUI Keybindings

| Key | Action |
|-----|--------|
| `q` | Quit |
| `d` | Change date range |
| `n/N` | Next/Previous port |
| `r` | Force refresh |
| `?` | Help |

## License

MIT
