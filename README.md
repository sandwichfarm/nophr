# nophr

**Nostr to Gopher/Gemini/Finger Gateway**

nophr is a personal gateway that serves your Nostr content via legacy internet protocols: Gopher (RFC 1436), Gemini, and Finger (RFC 742).

## Overview

- **Single-tenant** by default - shows one operator's notes and articles from Nostr
- **Config-first** - everything configurable via file and env overrides
- **Protocol servers** - Gopher, Gemini, and Finger simultaneously
- **Inbox/Outbox model** - aggregates replies, reactions, and zaps from Nostr
- **Smart relay discovery** - uses NIP-65 (kind 10002) for dynamic relay hints
- **Controlled sync scope** - sync self/following/mutual/FOAF with caps and deny lists
- **Embedded storage** - uses Khatru relay with SQLite or LMDB backend
- **Protocol-specific rendering** - gopher menus, gemini gemtext, finger responses

## Status

⚠️ **Early Development** - Not yet ready for production use.

Current implementation status:
- ✅ Configuration system with YAML parsing, validation, and display customization
- ✅ Storage layer with Khatru integration and SQLite backend
- ✅ Custom tables for relay hints, social graph, sync state, and aggregates
- ✅ Nostr client with relay discovery and event synchronization
- ✅ Protocol servers: Gopher (RFC 1436), Gemini (with TLS), and Finger (RFC 742)
- ✅ Content rendering with markdown support for all protocols
- ✅ Aggregates system for interaction tracking (reactions, zaps, replies)
- ✅ Display customization with headers, footers, and content filtering
- ✅ Comprehensive caching layer (memory and Redis support)

The core functionality is implemented and working. Focus is now on polish, optimization, and additional features.

## Quick Start

### Installation

#### Build from Source

```bash
# Clone repository
git clone https://github.com/sandwichfarm/nophr.git
cd nophr

# Build
make build

# Run
./dist/nophr --version
```

### Generate Configuration

```bash
# Generate example configuration
./dist/nophr init > nophr.yaml

# Edit with your npub and seed relays
vim nophr.yaml

# Validate configuration
./dist/nophr --config nophr.yaml
```

## Development

### Prerequisites

- Go 1.25 or later
- Make
- golangci-lint (for linting)

### Local Development

```bash
# Run all checks
make check

# Run tests
make test

# Run linters
make lint

# Build binary
make build

# Run locally
make dev
```

### Project Structure

```
nophr/
├── cmd/nophr/          # Main application
├── internal/            # Private application code
├── pkg/                 # Public libraries
├── configs/             # Example configurations
├── scripts/             # Build and CI scripts
├── memory/              # Design documentation
├── docs/                # User documentation
└── test/                # Integration tests
```

## Architecture

nophr follows a config-first philosophy with clear separation of concerns:

- **Storage Layer** - Khatru relay with SQLite/LMDB
- **Sync Engine** - Discovers and syncs from Nostr relays
- **Protocol Servers** - Gopher (port 70), Gemini (port 1965), Finger (port 79)
- **Rendering** - Protocol-specific content transformation
- **Caching** - In-memory or Redis for performance

For detailed architecture, see `memory/architecture.md`.

## Documentation

- `memory/configuration.md` - Configuration reference
- `memory/architecture.md` - System architecture
- `memory/storage_model.md` - Storage layer design
- `AGENTS.md` - Guidelines for contributors and AI agents

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

For AI agents working on this project, please read [AGENTS.md](AGENTS.md) first.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Related Projects

- [Khatru](https://github.com/fiatjaf/khatru) - Go relay framework
- [go-nostr](https://github.com/nbd-wtf/go-nostr) - Nostr protocol implementation

## Support

- Issues: https://github.com/sandwichfarm/nophr/issues
- Discussions: https://github.com/sandwichfarm/nophr/discussions

---

Built with ❤️ for the Nostr and legacy internet communities.
