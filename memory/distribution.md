Distribution Strategy

Goal: Make nophr as easy to install as possible across different platforms and use cases.

Single Binary Distribution

Yes! The entire application can be packaged as a single static binary with embedded resources.

Go Embed Feature (Go 1.16+)
- Use //go:embed directive to embed default configuration templates
- Embed example gophermap templates, gemtext templates
- Embed TLS certificate generation utilities
- No external file dependencies required (except user config and database)

Benefits:
- Zero dependencies: just download and run
- Cross-platform: compile for Linux, macOS, Windows, BSD
- Simple deployment: scp binary to server, chmod +x, run
- Easy updates: replace binary, restart service
- Perfect for gopherholes: minimalist distribution for minimalist protocols

What Goes in the Binary:
- Application code (sync engine, protocol servers, renderers)
- Embedded Khatru and eventstore libraries
- Default configuration template (nophr.example.yaml)
- Example layouts and section definitions
- Markdown conversion libraries
- TLS certificate generator for Gemini

What Stays External:
- User configuration file (nophr.yaml)
- Database files (SQLite or LMDB)
- TLS certificates (if user-provided)
- Logs

Distribution Channels

1. GitHub Releases (Primary)
- Pre-built binaries for major platforms via GoReleaser
- Platforms:
  - Linux: amd64, arm64, armv7
  - macOS: amd64 (Intel), arm64 (Apple Silicon)
  - FreeBSD: amd64, arm64
  - OpenBSD: amd64
  - Windows: amd64 (for testing/development)
- Archive formats: .tar.gz (Unix), .zip (Windows)
- Include: binary, example config, README, LICENSE
- Checksums (SHA256) for verification
- GPG signatures for security

2. Package Managers

Homebrew (macOS and Linux)
```bash
brew tap sandwich/nophr
brew install nophr
```
- GoReleaser auto-generates Homebrew formula
- Updates handled via brew upgrade
- Easy uninstall: brew uninstall nophr

APT (Debian/Ubuntu)
```bash
echo 'deb [trusted=yes] https://apt.nophr.io/ /' | sudo tee /etc/apt/sources.list.d/nophr.list
sudo apt update
sudo apt install nophr
```
- GoReleaser creates .deb packages
- Includes systemd service file
- Post-install script creates /etc/nophr/nophr.yaml template

RPM (Fedora/RHEL/Rocky/Alma)
```bash
sudo dnf config-manager --add-repo https://rpm.nophr.io/nophr.repo
sudo dnf install nophr
```
- GoReleaser creates .rpm packages
- Includes systemd service file

AUR (Arch Linux)
```bash
yay -S nophr-bin  # binary package
# or
yay -S nophr      # build from source
```
- Maintain PKGBUILD in AUR
- Community-maintained option

Snap (Ubuntu/Linux)
```bash
sudo snap install nophr
```
- Sandboxed environment
- Auto-updates by default
- May require connection permissions for network protocols

3. Docker / Container Images

Docker Hub
```bash
docker pull sandwich/nophr:latest
docker pull sandwich/nophr:v1.0.0
docker pull sandwich/nophr:v1-alpine
```

GitHub Container Registry
```bash
docker pull ghcr.io/sandwich/nophr:latest
```

Tags:
- latest: latest stable release
- v1.0.0: specific version
- v1-alpine: Alpine Linux base (smaller)
- edge: latest commit on main branch

Docker Compose Example:
```yaml
version: '3.8'
services:
  nophr:
    image: sandwich/nophr:latest
    ports:
      - "70:70"      # Gopher
      - "1965:1965"  # Gemini
      - "79:79"      # Finger
    volumes:
      - ./nophr.yaml:/etc/nophr/nophr.yaml:ro
      - ./data:/var/lib/nophr
      - ./certs:/etc/nophr/certs:ro
    environment:
      - NOPHR_NSEC=${NOPHR_NSEC}
    restart: unless-stopped
```

Multi-architecture images:
- linux/amd64
- linux/arm64
- linux/arm/v7

4. Building from Source

Go Toolchain Required:
```bash
# Clone repository
git clone https://github.com/sandwichfarm/nophr.git
cd nophr

# Build
go build -o nophr cmd/nophr/main.go

# Or use Makefile
make build

# Install to $GOPATH/bin
make install
```

Build Tags (optional features):
```bash
# Minimal build (SQLite only)
go build -tags sqlite

# LMDB support
go build -tags "sqlite lmdb"

# All features
go build -tags "sqlite lmdb debug"
```

5. One-Line Installers

Bash Script (Linux/macOS):
```bash
curl -fsSL https://get.nophr.io | sh
```

Script behavior:
- Detects OS and architecture
- Downloads appropriate binary from GitHub releases
- Verifies checksum
- Installs to /usr/local/bin (or ~/bin if no sudo)
- Creates example config at ~/.config/nophr/nophr.yaml
- Prints next steps

PowerShell Script (Windows):
```powershell
iwr -useb https://get.nophr.io/install.ps1 | iex
```

6. System Package Details

All packages include:
- Binary: /usr/bin/nophr
- Config: /etc/nophr/nophr.example.yaml
- Data dir: /var/lib/nophr (created, owned by nophr user)
- Systemd service: /etc/systemd/system/nophr.service
- Man page: /usr/share/man/man1/nophr.1.gz
- Documentation: /usr/share/doc/nophr/

Systemd Service File:
```ini
[Unit]
Description=nophr - Nostr to Gopher/Gemini/Finger Gateway
After=network.target
Documentation=https://github.com/sandwichfarm/nophr

[Service]
Type=simple
User=nophr
Group=nophr
WorkingDirectory=/var/lib/nophr
ExecStart=/usr/bin/nophr --config /etc/nophr/nophr.yaml
Restart=on-failure
RestartSec=5s

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/nophr
AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
```

Reverse Proxy Examples

Caddy (Recommended for Gemini TLS passthrough)

/etc/caddy/Caddyfile:
```
# Gopher proxy (if needed)
:70 {
    reverse_proxy localhost:7070
}

# Gemini - direct TLS passthrough (nophr handles TLS)
# No proxy needed - nophr listens on 1965 directly

# Optional: HTTPS admin panel
nophr-admin.example.com {
    reverse_proxy localhost:8080
}
```

Nginx (Gopher proxy if running non-privileged)

/etc/nginx/streams.d/gopher.conf:
```nginx
stream {
    upstream gopher_backend {
        server 127.0.0.1:7070;
    }

    server {
        listen 70;
        proxy_pass gopher_backend;
    }
}
```

Note: Nginx doesn't natively support Gopher or Gemini protocols.
Use for TCP proxying only if running nophr on non-privileged ports.

HAProxy (TCP load balancing for multiple instances)

/etc/haproxy/haproxy.cfg:
```
frontend gopher
    bind *:70
    mode tcp
    default_backend gopher_servers

backend gopher_servers
    mode tcp
    balance leastconn
    server nophr1 127.0.0.1:7070 check
    server nophr2 127.0.0.1:7071 check
```

systemd Socket Activation (Privilege separation)

Instead of running as root, use systemd socket activation:

/etc/systemd/system/nophr.socket:
```ini
[Unit]
Description=nophr Socket Activation

[Socket]
ListenStream=70
ListenStream=1965
ListenStream=79

[Install]
WantedBy=sockets.target
```

/etc/systemd/system/nophr.service:
```ini
[Unit]
Description=nophr Service
Requires=nophr.socket

[Service]
Type=simple
User=nophr
ExecStart=/usr/bin/nophr --systemd-socket
StandardInput=socket
```

This allows nophr to run as unprivileged user while binding to privileged ports.

GoReleaser Configuration

.goreleaser.yaml:
```yaml
before:
  hooks:
    - go mod tidy
    - go generate ./...

builds:
  - id: nophr
    main: ./cmd/nophr
    binary: nophr
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - freebsd
      - openbsd
    goarch:
      - amd64
      - arm64
      - arm
    goarm:
      - "7"
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}

archives:
  - format: tar.gz
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}
      {{- if .Arm }}v{{ .Arm }}{{ end }}
    files:
      - README.md
      - LICENSE
      - configs/nophr.example.yaml
      - docs/*

checksum:
  name_template: 'checksums.txt'

snapshot:
  name_template: "{{ incpatch .Version }}-next"

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'

brews:
  - name: nophr
    tap:
      owner: sandwich
      name: homebrew-nophr
    homepage: https://github.com/sandwichfarm/nophr
    description: "Nostr to Gopher/Gemini/Finger gateway"
    license: MIT
    install: |
      bin.install "nophr"
      etc.install "configs/nophr.example.yaml" => "nophr.example.yaml"
    test: |
      system "#{bin}/nophr", "--version"

nfpms:
  - id: nophr-packages
    package_name: nophr
    homepage: https://github.com/sandwichfarm/nophr
    maintainer: Your Name <you@example.com>
    description: Nostr to Gopher/Gemini/Finger gateway
    license: MIT
    formats:
      - deb
      - rpm
      - apk
    bindir: /usr/bin
    contents:
      - src: configs/nophr.example.yaml
        dst: /etc/nophr/nophr.example.yaml
        type: config
      - src: scripts/systemd/nophr.service
        dst: /etc/systemd/system/nophr.service
        type: config
    scripts:
      postinstall: scripts/postinstall.sh

dockers:
  - image_templates:
      - "sandwich/nophr:{{ .Version }}-amd64"
      - "sandwich/nophr:latest-amd64"
      - "ghcr.io/sandwich/nophr:{{ .Version }}-amd64"
      - "ghcr.io/sandwich/nophr:latest-amd64"
    dockerfile: Dockerfile
    use: buildx
    build_flag_templates:
      - "--platform=linux/amd64"
      - "--label=org.opencontainers.image.title={{ .ProjectName }}"
      - "--label=org.opencontainers.image.version={{ .Version }}"

docker_manifests:
  - name_template: sandwich/nophr:{{ .Version }}
    image_templates:
      - sandwich/nophr:{{ .Version }}-amd64
      - sandwich/nophr:{{ .Version }}-arm64
  - name_template: sandwich/nophr:latest
    image_templates:
      - sandwich/nophr:latest-amd64
      - sandwich/nophr:latest-arm64
```

Dockerfile (Multi-stage)

```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-s -w" \
    -o nophr cmd/nophr/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

RUN addgroup -g 1000 nophr && \
    adduser -D -u 1000 -G nophr nophr

WORKDIR /app

COPY --from=builder /build/nophr /usr/local/bin/nophr
COPY configs/nophr.example.yaml /etc/nophr/nophr.example.yaml

RUN mkdir -p /var/lib/nophr /etc/nophr/certs && \
    chown -R nophr:nophr /var/lib/nophr /etc/nophr

USER nophr

VOLUME ["/var/lib/nophr", "/etc/nophr"]

EXPOSE 70 1965 79

ENTRYPOINT ["/usr/local/bin/nophr"]
CMD ["--config", "/etc/nophr/nophr.yaml"]
```

Documentation for Users

Quick Start Guide (in README.md):

1. Download binary:
   ```bash
   # Linux/macOS one-liner
   curl -fsSL https://get.nophr.io | sh

   # Or download from releases
   wget https://github.com/sandwich/nophr/releases/download/v1.0.0/nophr_1.0.0_linux_amd64.tar.gz
   tar xzf nophr_1.0.0_linux_amd64.tar.gz
   sudo mv nophr /usr/local/bin/
   ```

2. Create config:
   ```bash
   mkdir -p ~/.config/nophr
   nophr init > ~/.config/nophr/nophr.yaml
   # Edit config with your npub and seed relays
   ```

3. Run:
   ```bash
   nophr --config ~/.config/nophr/nophr.yaml
   ```

4. Test:
   ```bash
   # Gopher
   lynx gopher://localhost

   # Gemini
   amfora gemini://localhost

   # Finger
   finger @localhost
   ```

Installation Matrix

| Method | Platforms | Auto-update | Privileges | Complexity |
|--------|-----------|-------------|------------|------------|
| Binary | All | Manual | User-managed | Lowest |
| Homebrew | macOS/Linux | brew upgrade | User | Low |
| APT/RPM | Linux | apt/dnf | Root | Low |
| Snap | Linux | Automatic | Confined | Low |
| Docker | All | Pull new image | User/root | Medium |
| Source | All | Manual | User | High |

Recommended by Use Case:
- Personal gopherhole: Single binary or Homebrew
- VPS deployment: APT/RPM packages with systemd
- Homelab: Docker Compose
- Development: Build from source
- Multi-instance: Docker Swarm or Kubernetes

Summary

✅ Single binary: Yes, with embedded resources via //go:embed
✅ Package managers: Homebrew, APT, RPM, Snap, AUR
✅ Containers: Docker multi-arch images
✅ One-line installer: Bash/PowerShell scripts
✅ Source: Standard Go build process
✅ Reverse proxies: Example configs for Caddy, Nginx, HAProxy
✅ Systemd integration: Socket activation, service hardening

The Go ecosystem makes all of this achievable with GoReleaser automating most of the distribution pipeline.
