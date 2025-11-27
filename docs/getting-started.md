# Getting Started with nophr

nophr is a personal gateway that serves your Nostr content via legacy internet protocols: Gopher (RFC 1436), Gemini, and Finger (RFC 742).

This guide covers installation, initial configuration, and first run.

## Prerequisites

**For one-line install:** curl or wget

**For building from source:**
- **Go 1.25 or later**
- **Make** - for build automation
- **Git** - for cloning the repository
- **golangci-lint** (optional) - for running linters during development

## Installation

### Quick Install (Recommended)

Use the one-line installer:

```bash
curl -sSL https://nophr.io/install.sh | sh
```

This will:
- Detect your platform and architecture
- Download the latest release
- Install to `/usr/local/bin/nophr`
- Create example configuration

**For more installation options** (Docker, packages, etc.), see [installation.md](installation.md).

### Build from Source

```bash
# Clone the repository
git clone https://github.com/sandwichfarm/nophr.git
cd nophr

# Build the binary
make build

# The binary will be in dist/nophr
./dist/nophr --version
```

You should see output like:
```
nophr dev
  commit: abc1234
  built:  2025-10-24T12:00:00Z
  by:     manual
```

### Install System-Wide (Optional)

```bash
# Install to /usr/local/bin (requires sudo)
sudo make install

# Verify installation
nophr --version
```

## Initial Configuration

nophr uses a YAML configuration file. Generate an example configuration:

```bash
# Generate example configuration
./dist/nophr init > nophr.yaml
```

### Minimum Required Configuration

Edit `nophr.yaml` and configure these essential settings:

```yaml
site:
  title: "My Nostr Site"
  description: "Personal Nostr gateway"
  operator: "Your Name"

identity:
  npub: "npub1..." # Your Nostr public key (REQUIRED)

relays:
  seeds:
    - "wss://relay.damus.io"
    - "wss://relay.nostr.band"
    - "wss://nos.lol"
```

**Important:** You must provide your `npub` (Nostr public key). This identifies whose content to serve.

 

## Validate Configuration

Test that your configuration is valid:

```bash
./dist/nophr --config nophr.yaml
```

If configuration is valid, you should see:
```
Starting nophr dev
  Site: My Nostr Site
  Operator: Your Name
  Identity: npub1...

Initializing storage...
  Storage: sqlite initialized
Initializing aggregates manager...
  Aggregates manager ready
Initializing sync engine...
  Sync engine started
Starting Gopher server on localhost:70...
  Gopher server ready
Starting Gemini server on localhost:1965...
  Gemini server ready
Starting Finger server on port 79...
  Finger server ready

✓ All services started successfully!
```

 

## Understanding the Configuration

The generated configuration includes many sections, such as:

- `site` — Site metadata (title, description)
- `identity` — Your npub (public key)
- `protocols` — Enable/disable Gopher, Gemini, Finger
- `relays` — Seed relays and connection policies
- `discovery` — Relay discovery settings (NIP-65)
- `sync` — Event synchronization (enabled/scope/retention)
- `inbox` — Aggregation of replies/reactions/zaps
- `storage` — Database backend (SQLite)
- `rendering` — Protocol-specific rendering options
- `caching` — Response caching (memory/Redis)
- `logging` — Log level configuration
- `sections` — Custom filtered views at specific URL paths
- `security` — Security features (deny lists, rate limiting, validation)
- `display` — Display control (feed/detail views, limits)
- `presentation` — Visual presentation (headers, footers, separators)
- `behavior` — Behavior control (filtering, sorting)

For complete configuration documentation, see [configuration.md](configuration.md).

For security guidance, see [security.md](security.md).

## Storage Backend

nophr stores Nostr events in a local database using [Khatru](https://github.com/fiatjaf/khatru) (embedded Nostr relay).

**Default:** SQLite at `./data/nophr.db`

```yaml
storage:
  driver: "sqlite"
  sqlite_path: "./data/nophr.db"
```

The database file will be created automatically on first run.

For more on storage backends, see [storage.md](storage.md).

## Next Steps

Now that you have nophr configured:

1. **Test Protocol Servers**:
   - Connect to Gopher: `telnet localhost 70` (or use a Gopher client like lynx/VF-1)
   - Connect to Gemini: Use a Gemini client like amfora or lagrange
   - Connect to Finger: `finger @localhost` (or `telnet localhost 79`)

2. **Understand Nostr Integration**:
   - Learn how sync works: [nostr-integration.md](nostr-integration.md)
   - Configure sync scope (self/following/mutual/FOAF)

3. **Customize Rendering**:
   - Adjust line lengths, timestamps, formatting: [protocols.md](protocols.md)

4. **Deploy to Production**:
   - Run as systemd service, configure ports: [deployment.md](deployment.md)

## Development

### Run Tests

```bash
make test
```

### Run Linters

```bash
make lint
```

### Run All Checks

```bash
make check
```

### Development Mode

Run directly from source with live reloading:

```bash
make dev
```

This runs `go run ./cmd/nophr --config ./configs/nophr.example.yaml`.

## Project Structure

```
nophr/
├── cmd/nophr/          # Main application entry point
├── internal/            # Private application code
│   ├── config/          # Configuration loading and validation
│   ├── storage/         # Storage layer (SQLite/LMDB)
│   ├── nostr/           # Nostr client and relay discovery
│   ├── sync/            # Event synchronization engine
│   ├── aggregates/      # Interaction aggregation (replies, zaps, etc.)
│   ├── markdown/        # Markdown to protocol conversion
│   ├── gopher/          # Gopher protocol server
│   ├── gemini/          # Gemini protocol server
│   └── finger/          # Finger protocol server
├── configs/             # Example configurations
├── memory/              # Design documentation (for contributors)
├── docs/                # User documentation (you are here)
└── scripts/             # Build and CI scripts
```

## Troubleshooting

### "identity.npub is required"

You forgot to set your `npub` in `nophr.yaml`. Get your npub from any Nostr client.

### "failed to initialize storage: unable to open database file"

The directory for the database doesn't exist. Create it:
```bash
mkdir -p ./data
```

### "port already in use"

Another service is using one of the protocol ports (70, 79, or 1965). Either:
- Stop the conflicting service
- Change the port in `nophr.yaml`
- Disable that protocol

### "permission denied" binding to port

Ports below 1024 require root/sudo permissions. Either:
- Run with sudo: `sudo ./dist/nophr --config nophr.yaml`
- Use port forwarding: `iptables` to forward 70→7070, etc.
- Change ports in config to >1024 (testing only)

For more troubleshooting, see [troubleshooting.md](troubleshooting.md).

## Getting Help

- **Documentation:** Browse docs/ for detailed guides
- **Issues:** Report bugs at https://github.com/sandwichfarm/nophr/issues
- **Discussions:** Ask questions at https://github.com/sandwichfarm/nophr/discussions
- **Design Docs:** See memory/ for technical design decisions

## Contributing

Contributions welcome! See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.

For AI agents working on this project, see [AGENTS.md](../AGENTS.md).

---

**Next:** [Configuration Reference](configuration.md) | [Storage Guide](storage.md) | [Architecture Overview](architecture.md)
