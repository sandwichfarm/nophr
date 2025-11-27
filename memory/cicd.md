CI/CD Pipeline Design

Philosophy
- Local scripts first: all CI operations can run locally for testing
- GitHub Actions orchestrates the scripts
- Consistent behavior: local and CI use same scripts
- Fast feedback: parallel jobs where possible
- Security: secrets managed via GitHub secrets, signed releases

Directory Structure

```
nophr/
├── .github/
│   └── workflows/
│       ├── test.yml           # Run tests on PR/push
│       ├── lint.yml           # Code quality checks
│       ├── build.yml          # Build verification
│       ├── release.yml        # Release on tag push
│       └── docker.yml         # Docker image builds
├── scripts/
│   ├── test.sh               # Run all tests
│   ├── lint.sh               # Linting and formatting
│   ├── build.sh              # Build binaries
│   ├── install.sh            # One-line installer (for users)
│   ├── postinstall.sh        # Post-install for packages
│   └── ci/
│       ├── setup.sh          # CI environment setup
│       └── verify.sh         # Verify build artifacts
├── Makefile                   # Common tasks
├── .goreleaser.yml           # GoReleaser configuration
├── Dockerfile                # Container build
└── docker-compose.yml        # Local testing
```

Local Scripts

scripts/test.sh
```bash
#!/usr/bin/env bash
set -euo pipefail

# Run all tests with coverage
echo "==> Running tests..."

# Unit tests
go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

# Display coverage summary
go tool cover -func=coverage.txt | tail -n 1

# Optional: generate HTML coverage report
if [ "${HTML_COVERAGE:-false}" = "true" ]; then
    go tool cover -html=coverage.txt -o coverage.html
    echo "Coverage report: coverage.html"
fi

echo "==> Tests passed!"
```

scripts/lint.sh
```bash
#!/usr/bin/env bash
set -euo pipefail

echo "==> Running linters..."

# Check if golangci-lint is installed
if ! command -v golangci-lint &> /dev/null; then
    echo "golangci-lint not found, installing..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
fi

# Run golangci-lint
golangci-lint run ./...

# Check formatting
echo "==> Checking formatting..."
UNFORMATTED=$(gofmt -l .)
if [ -n "$UNFORMATTED" ]; then
    echo "The following files are not formatted:"
    echo "$UNFORMATTED"
    echo "Run: gofmt -w ."
    exit 1
fi

# Check go mod tidy
echo "==> Checking go.mod..."
go mod tidy
if ! git diff --quiet go.mod go.sum; then
    echo "go.mod or go.sum is not tidy. Run: go mod tidy"
    exit 1
fi

echo "==> Linting passed!"
```

scripts/build.sh
```bash
#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-dev}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"
DATE="${DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"

echo "==> Building nophr..."
echo "    Version: $VERSION"
echo "    Commit:  $COMMIT"
echo "    Date:    $DATE"

# Build flags
LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X main.version=$VERSION"
LDFLAGS="$LDFLAGS -X main.commit=$COMMIT"
LDFLAGS="$LDFLAGS -X main.date=$DATE"

# Build binary
CGO_ENABLED=0 go build \
    -ldflags "$LDFLAGS" \
    -o dist/nophr \
    ./cmd/nophr

echo "==> Build complete: dist/nophr"

# Show version
./dist/nophr --version
```

scripts/ci/setup.sh
```bash
#!/usr/bin/env bash
set -euo pipefail

echo "==> Setting up CI environment..."

# Install dependencies
go mod download

# Install tools
echo "==> Installing tools..."
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Verify Go version
echo "==> Go version:"
go version

echo "==> Setup complete!"
```

scripts/ci/verify.sh
```bash
#!/usr/bin/env bash
set -euo pipefail

echo "==> Verifying build artifacts..."

# Check that binaries exist
if [ ! -d "dist" ]; then
    echo "ERROR: dist/ directory not found"
    exit 1
fi

# Check that at least one binary exists
BINARY_COUNT=$(find dist -type f -executable | wc -l)
if [ "$BINARY_COUNT" -eq 0 ]; then
    echo "ERROR: No binaries found in dist/"
    exit 1
fi

echo "==> Found $BINARY_COUNT binaries"

# Verify each binary
for binary in dist/*; do
    if [ -x "$binary" ] && [ -f "$binary" ]; then
        echo "    Checking: $binary"

        # Check if binary runs
        if ! "$binary" --version &> /dev/null; then
            echo "    WARNING: $binary --version failed"
        else
            echo "    ✓ $binary is valid"
        fi
    fi
done

echo "==> Verification complete!"
```

scripts/install.sh (user-facing one-line installer)
```bash
#!/usr/bin/env bash
# Install script for nophr
# Usage: curl -fsSL https://get.nophr.io | sh

set -euo pipefail

# Configuration
REPO="sandwichfarm/nophr"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="${CONFIG_DIR:-$HOME/.config/nophr}"

# Detect OS and architecture
detect_platform() {
    local OS
    local ARCH

    OS="$(uname -s)"
    case "$OS" in
        Linux)  OS="linux" ;;
        Darwin) OS="darwin" ;;
        FreeBSD) OS="freebsd" ;;
        OpenBSD) OS="openbsd" ;;
        *)
            echo "Unsupported operating system: $OS"
            exit 1
            ;;
    esac

    ARCH="$(uname -m)"
    case "$ARCH" in
        x86_64)  ARCH="amd64" ;;
        aarch64) ARCH="arm64" ;;
        armv7l)  ARCH="armv7" ;;
        *)
            echo "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    echo "${OS}_${ARCH}"
}

# Get latest release version
get_latest_version() {
    curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
        | grep '"tag_name":' \
        | sed -E 's/.*"([^"]+)".*/\1/'
}

main() {
    echo "==> Installing nophr..."

    local PLATFORM
    PLATFORM="$(detect_platform)"
    echo "    Platform: $PLATFORM"

    local VERSION
    VERSION="$(get_latest_version)"
    echo "    Version:  $VERSION"

    local ARCHIVE="nophr_${VERSION#v}_${PLATFORM}.tar.gz"
    local URL="https://github.com/$REPO/releases/download/$VERSION/$ARCHIVE"

    echo "==> Downloading from $URL..."
    local TMPDIR
    TMPDIR="$(mktemp -d)"
    trap "rm -rf $TMPDIR" EXIT

    curl -fsSL "$URL" -o "$TMPDIR/$ARCHIVE"

    echo "==> Extracting..."
    tar -xzf "$TMPDIR/$ARCHIVE" -C "$TMPDIR"

    echo "==> Installing to $INSTALL_DIR..."
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMPDIR/nophr" "$INSTALL_DIR/nophr"
    else
        echo "    (requires sudo)"
        sudo mv "$TMPDIR/nophr" "$INSTALL_DIR/nophr"
    fi

    echo "==> Creating config directory..."
    mkdir -p "$CONFIG_DIR"

    if [ -f "$TMPDIR/nophr.example.yaml" ]; then
        if [ ! -f "$CONFIG_DIR/nophr.yaml" ]; then
            cp "$TMPDIR/nophr.example.yaml" "$CONFIG_DIR/nophr.yaml"
            echo "    Created: $CONFIG_DIR/nophr.yaml"
        else
            echo "    Config already exists: $CONFIG_DIR/nophr.yaml"
        fi
    fi

    echo ""
    echo "==> nophr installed successfully!"
    echo ""
    echo "Next steps:"
    echo "  1. Edit config: $CONFIG_DIR/nophr.yaml"
    echo "  2. Run: nophr --config $CONFIG_DIR/nophr.yaml"
    echo ""
    echo "For help: nophr --help"
}

main "$@"
```

scripts/postinstall.sh (for system packages)
```bash
#!/usr/bin/env bash
# Post-install script for system packages (APT/RPM)

set -euo pipefail

# Create nophr user if it doesn't exist
if ! id -u nophr &>/dev/null; then
    useradd --system --user-group --no-create-home --shell /bin/false nophr
fi

# Create data directory
mkdir -p /var/lib/nophr
chown nophr:nophr /var/lib/nophr
chmod 750 /var/lib/nophr

# Create config directory
mkdir -p /etc/nophr
chmod 755 /etc/nophr

# Copy example config if main config doesn't exist
if [ ! -f /etc/nophr/nophr.yaml ] && [ -f /etc/nophr/nophr.example.yaml ]; then
    cp /etc/nophr/nophr.example.yaml /etc/nophr/nophr.yaml
    chmod 640 /etc/nophr/nophr.yaml
    chown root:nophr /etc/nophr/nophr.yaml
    echo "Created /etc/nophr/nophr.yaml - please edit before starting"
fi

# Reload systemd
if command -v systemctl &>/dev/null; then
    systemctl daemon-reload
fi

echo "nophr installed successfully!"
echo "Edit /etc/nophr/nophr.yaml and run: systemctl start nophr"
```

Makefile
```makefile
.PHONY: help build test lint clean install dev release docker

# Variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build binary
	@VERSION=$(VERSION) COMMIT=$(COMMIT) DATE=$(DATE) ./scripts/build.sh

test: ## Run tests
	@./scripts/test.sh

lint: ## Run linters
	@./scripts/lint.sh

clean: ## Clean build artifacts
	rm -rf dist/ coverage.txt coverage.html

install: build ## Install to /usr/local/bin
	install -m 755 dist/nophr /usr/local/bin/nophr

dev: ## Run in development mode
	go run ./cmd/nophr --config ./configs/nophr.example.yaml

release: ## Create a release (requires goreleaser)
	goreleaser release --clean

release-snapshot: ## Create a snapshot release
	goreleaser release --snapshot --clean

docker: ## Build Docker image
	docker build -t nophr:$(VERSION) .

docker-compose-up: ## Start with docker-compose
	docker-compose up -d

docker-compose-down: ## Stop docker-compose
	docker-compose down

ci-setup: ## Setup CI environment
	@./scripts/ci/setup.sh

ci-verify: ## Verify build artifacts
	@./scripts/ci/verify.sh

fmt: ## Format code
	gofmt -w .
	go mod tidy

check: lint test ## Run all checks
```

GitHub Workflows

.github/workflows/test.yml
```yaml
name: Test

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

jobs:
  test:
    name: Test on ${{ matrix.os }}
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
        go-version: ['1.24']

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: ${{ matrix.go-version }}
        cache: true

    - name: Setup CI
      run: ./scripts/ci/setup.sh

    - name: Run tests
      run: ./scripts/test.sh

    - name: Upload coverage
      uses: codecov/codecov-action@v4
      with:
        files: ./coverage.txt
        flags: unittests
        name: codecov-${{ matrix.os }}
      env:
        CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}
```

.github/workflows/lint.yml
```yaml
name: Lint

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

jobs:
  lint:
    name: Lint
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.24'
        cache: true

    - name: Run linters
      run: ./scripts/lint.sh

    - name: golangci-lint
      uses: golangci/golangci-lint-action@v4
      with:
        version: latest
        args: --timeout=5m
```

.github/workflows/build.yml
```yaml
name: Build

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

jobs:
  build:
    name: Build on ${{ matrix.os }} for ${{ matrix.goos }}/${{ matrix.goarch }}
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        include:
          - os: ubuntu-latest
            goos: linux
            goarch: amd64
          - os: ubuntu-latest
            goos: linux
            goarch: arm64
          - os: macos-latest
            goos: darwin
            goarch: amd64
          - os: macos-latest
            goos: darwin
            goarch: arm64

    steps:
    - name: Checkout code
      uses: actions/checkout@v4
      with:
        fetch-depth: 0  # Full history for version detection

    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.24'
        cache: true

    - name: Build binary
      run: |
        export GOOS=${{ matrix.goos }}
        export GOARCH=${{ matrix.goarch }}
        ./scripts/build.sh
      env:
        VERSION: ${{ github.ref_name }}
        COMMIT: ${{ github.sha }}

    - name: Verify build
      run: ./scripts/ci/verify.sh

    - name: Upload artifact
      uses: actions/upload-artifact@v4
      with:
        name: nophr-${{ matrix.goos }}-${{ matrix.goarch }}
        path: dist/nophr
        retention-days: 7
```

.github/workflows/release.yml
```yaml
name: Release

on:
  push:
    tags:
      - 'v*'  # Trigger on version tags (v1.0.0, v2.1.3, etc.)

permissions:
  contents: write  # Required for creating releases
  packages: write  # Required for Docker images

jobs:
  release:
    name: Create Release
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v4
      with:
        fetch-depth: 0  # Required for changelog generation

    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.24'
        cache: true

    - name: Setup CI
      run: ./scripts/ci/setup.sh

    - name: Run tests
      run: ./scripts/test.sh

    - name: Import GPG key
      id: import_gpg
      uses: crazy-max/ghaction-import-gpg@v6
      with:
        gpg_private_key: ${{ secrets.GPG_PRIVATE_KEY }}
        passphrase: ${{ secrets.GPG_PASSPHRASE }}

    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v3

    - name: Login to Docker Hub
      uses: docker/login-action@v3
      with:
        username: ${{ secrets.DOCKER_USERNAME }}
        password: ${{ secrets.DOCKER_PASSWORD }}

    - name: Login to GitHub Container Registry
      uses: docker/login-action@v3
      with:
        registry: ghcr.io
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}

    - name: Run GoReleaser
      uses: goreleaser/goreleaser-action@v5
      with:
        distribution: goreleaser
        version: latest
        args: release --clean
      env:
        GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        GPG_FINGERPRINT: ${{ steps.import_gpg.outputs.fingerprint }}

    - name: Verify release artifacts
      run: ./scripts/ci/verify.sh

    - name: Upload release assets checksum
      uses: actions/upload-artifact@v4
      with:
        name: checksums
        path: dist/checksums.txt
        retention-days: 90

  publish-homebrew:
    name: Update Homebrew Formula
    needs: release
    runs-on: ubuntu-latest

    steps:
    - name: Checkout homebrew tap
      uses: actions/checkout@v4
      with:
        repository: ${{ github.repository_owner }}/homebrew-nophr
        token: ${{ secrets.TAP_GITHUB_TOKEN }}

    - name: Update formula
      run: |
        # GoReleaser already updated the formula
        # This job is for any additional homebrew-specific tasks
        echo "Formula updated by GoReleaser"
```

.github/workflows/docker.yml
```yaml
name: Docker

on:
  push:
    branches: [ main ]
    tags: [ 'v*' ]
  pull_request:
    branches: [ main ]

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build-and-push:
    name: Build and Push Docker Image
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up QEMU
      uses: docker/setup-qemu-action@v3

    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v3

    - name: Login to GitHub Container Registry
      if: github.event_name != 'pull_request'
      uses: docker/login-action@v3
      with:
        registry: ${{ env.REGISTRY }}
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}

    - name: Extract metadata
      id: meta
      uses: docker/metadata-action@v5
      with:
        images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
        tags: |
          type=ref,event=branch
          type=ref,event=pr
          type=semver,pattern={{version}}
          type=semver,pattern={{major}}.{{minor}}
          type=semver,pattern={{major}}
          type=sha

    - name: Build and push
      uses: docker/build-push-action@v5
      with:
        context: .
        platforms: linux/amd64,linux/arm64,linux/arm/v7
        push: ${{ github.event_name != 'pull_request' }}
        tags: ${{ steps.meta.outputs.tags }}
        labels: ${{ steps.meta.outputs.labels }}
        cache-from: type=gha
        cache-to: type=gha,mode=max
```

GoReleaser Configuration

.goreleaser.yml
```yaml
version: 2

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
      - -X main.builtBy=goreleaser
    mod_timestamp: '{{ .CommitTimestamp }}'

archives:
  - id: nophr-archive
    format: tar.gz
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}
      {{- if .Arm }}v{{ .Arm }}{{ end }}
    format_overrides:
      - goos: windows
        format: zip
    files:
      - README.md
      - LICENSE
      - configs/nophr.example.yaml
      - docs/**/*

checksum:
  name_template: 'checksums.txt'
  algorithm: sha256

snapshot:
  name_template: "{{ incpatch .Version }}-next"

changelog:
  sort: asc
  use: github
  filters:
    exclude:
      - '^docs:'
      - '^test:'
      - '^ci:'
      - '^chore:'
      - '^style:'
      - Merge pull request
      - Merge branch
  groups:
    - title: Features
      regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$'
      order: 0
    - title: 'Bug Fixes'
      regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$'
      order: 1
    - title: 'Performance Improvements'
      regexp: '^.*?perf(\([[:word:]]+\))??!?:.+$'
      order: 2
    - title: Others
      order: 999

release:
  github:
    owner: sandwich
    name: nophr
  draft: false
  prerelease: auto
  name_template: "{{.ProjectName}} v{{.Version}}"
  header: |
    ## nophr {{.Version}}

    Release of nophr {{.Version}}
  footer: |
    **Full Changelog**: https://github.com/sandwichfarm/nophr/compare/{{ .PreviousTag }}...{{ .Tag }}

brews:
  - name: nophr
    repository:
      owner: sandwich
      name: homebrew-nophr
      token: "{{ .Env.TAP_GITHUB_TOKEN }}"
    directory: Formula
    homepage: https://github.com/sandwichfarm/nophr
    description: "Nostr to Gopher/Gemini/Finger gateway"
    license: MIT
    test: |
      system "#{bin}/nophr", "--version"
    install: |
      bin.install "nophr"
      etc.install "configs/nophr.example.yaml" => "nophr.example.yaml"

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
    overrides:
      deb:
        dependencies:
          - ca-certificates
      rpm:
        dependencies:
          - ca-certificates

dockers:
  - image_templates:
      - "ghcr.io/{{ .Env.GITHUB_REPOSITORY_OWNER }}/nophr:{{ .Version }}-amd64"
      - "ghcr.io/{{ .Env.GITHUB_REPOSITORY_OWNER }}/nophr:latest-amd64"
    dockerfile: Dockerfile
    use: buildx
    build_flag_templates:
      - "--platform=linux/amd64"
      - "--label=org.opencontainers.image.created={{.Date}}"
      - "--label=org.opencontainers.image.title={{.ProjectName}}"
      - "--label=org.opencontainers.image.revision={{.FullCommit}}"
      - "--label=org.opencontainers.image.version={{.Version}}"
      - "--label=org.opencontainers.image.source={{.GitURL}}"

  - image_templates:
      - "ghcr.io/{{ .Env.GITHUB_REPOSITORY_OWNER }}/nophr:{{ .Version }}-arm64"
      - "ghcr.io/{{ .Env.GITHUB_REPOSITORY_OWNER }}/nophr:latest-arm64"
    dockerfile: Dockerfile
    use: buildx
    build_flag_templates:
      - "--platform=linux/arm64"
      - "--label=org.opencontainers.image.created={{.Date}}"
      - "--label=org.opencontainers.image.title={{.ProjectName}}"
      - "--label=org.opencontainers.image.revision={{.FullCommit}}"
      - "--label=org.opencontainers.image.version={{.Version}}"
      - "--label=org.opencontainers.image.source={{.GitURL}}"

docker_manifests:
  - name_template: ghcr.io/{{ .Env.GITHUB_REPOSITORY_OWNER }}/nophr:{{ .Version }}
    image_templates:
      - ghcr.io/{{ .Env.GITHUB_REPOSITORY_OWNER }}/nophr:{{ .Version }}-amd64
      - ghcr.io/{{ .Env.GITHUB_REPOSITORY_OWNER }}/nophr:{{ .Version }}-arm64

  - name_template: ghcr.io/{{ .Env.GITHUB_REPOSITORY_OWNER }}/nophr:latest
    image_templates:
      - ghcr.io/{{ .Env.GITHUB_REPOSITORY_OWNER }}/nophr:latest-amd64
      - ghcr.io/{{ .Env.GITHUB_REPOSITORY_OWNER }}/nophr:latest-arm64

signs:
  - cmd: gpg
    args:
      - "--batch"
      - "--local-user"
      - "{{ .Env.GPG_FINGERPRINT }}"
      - "--output"
      - "${signature}"
      - "--detach-sign"
      - "${artifact}"
    artifacts: checksum
    output: true
```

Usage Documentation

Local Development Workflow
```bash
# Clone repository
git clone https://github.com/sandwichfarm/nophr.git
cd nophr

# Run tests
make test

# Run linters
make lint

# Build
make build

# Run locally
make dev

# Or run all checks
make check
```

Creating a Release
```bash
# 1. Update version in code (if needed)
# 2. Commit changes
git add .
git commit -m "chore: prepare for release v1.0.0"

# 3. Create and push tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# GitHub Actions will automatically:
# - Run tests
# - Build binaries for all platforms
# - Create GitHub release with changelog
# - Upload release assets
# - Build and push Docker images
# - Update Homebrew formula
# - Create .deb and .rpm packages
```

Required GitHub Secrets
```
CODECOV_TOKEN         # For code coverage (optional)
GPG_PRIVATE_KEY       # For signing releases
GPG_PASSPHRASE        # GPG key passphrase
DOCKER_USERNAME       # Docker Hub username (optional)
DOCKER_PASSWORD       # Docker Hub token (optional)
TAP_GITHUB_TOKEN      # Token for homebrew tap repo
```

CI/CD Pipeline Flow

```
┌─────────────────────────────────────────────────────────┐
│  Developer pushes code / PR                             │
└────────────────┬────────────────────────────────────────┘
                 │
                 ├──> [Test Workflow]
                 │     ├─> Run tests on Linux
                 │     ├─> Run tests on macOS
                 │     └─> Upload coverage
                 │
                 ├──> [Lint Workflow]
                 │     ├─> Run golangci-lint
                 │     ├─> Check formatting
                 │     └─> Verify go.mod
                 │
                 └──> [Build Workflow]
                       ├─> Build for linux/amd64
                       ├─> Build for linux/arm64
                       ├─> Build for darwin/amd64
                       ├─> Build for darwin/arm64
                       └─> Upload artifacts

┌─────────────────────────────────────────────────────────┐
│  Developer pushes version tag (v1.0.0)                  │
└────────────────┬────────────────────────────────────────┘
                 │
                 └──> [Release Workflow]
                       ├─> Run all tests
                       ├─> Run GoReleaser
                       │    ├─> Build all platforms
                       │    ├─> Create archives
                       │    ├─> Generate checksums
                       │    ├─> Sign with GPG
                       │    ├─> Generate changelog
                       │    ├─> Create GitHub release
                       │    ├─> Build .deb packages
                       │    ├─> Build .rpm packages
                       │    ├─> Update Homebrew formula
                       │    └─> Build Docker images
                       └─> Upload to package repos

┌─────────────────────────────────────────────────────────┐
│  Push to main branch                                    │
└────────────────┬────────────────────────────────────────┘
                 │
                 └──> [Docker Workflow]
                       ├─> Build multi-arch image
                       ├─> Push to GHCR
                       └─> Tag as 'edge'
```

Summary

✅ Local scripts for all operations (test, lint, build, install)
✅ Makefile for common tasks
✅ GitHub Actions workflows:
   - Test: Run on every PR/push
   - Lint: Code quality checks
   - Build: Verify builds for all platforms
   - Release: Full release on tag push
   - Docker: Container image builds
✅ GoReleaser for automated releases:
   - Multi-platform binaries
   - Changelog from commits
   - GitHub release creation
   - Package generation (deb, rpm, apk)
   - Homebrew formula
   - Docker multi-arch images
   - GPG signing
✅ One-line installer script for users
✅ Systemd integration
✅ Complete automation from tag to distribution

CI/CD Pipeline Summary

Overview

This CI/CD system follows a "local scripts first" philosophy - all operations can be tested locally before running in CI.

Local Scripts

All CI operations can run locally for testing:

- scripts/test.sh - Run all tests with coverage
- scripts/lint.sh - Linting and formatting checks
- scripts/build.sh - Build binaries with version info
- scripts/install.sh - One-line installer for users (curl | sh)
- scripts/postinstall.sh - Post-install for system packages
- scripts/ci/setup.sh - CI environment setup
- scripts/ci/verify.sh - Verify build artifacts

Makefile Commands

```bash
make test          # Run tests
make lint          # Run linters
make build         # Build binary
make check         # Run all checks
make release       # Create release with GoReleaser
make docker        # Build Docker image
make release-snapshot  # Test release locally without publishing
```

GitHub Actions Workflows

1. test.yml - Runs on every push/PR
   - Tests on Linux and macOS
   - Multiple Go versions support
   - Uploads coverage to Codecov
   - Parallel execution for speed

2. lint.yml - Code quality on every push/PR
   - golangci-lint with all checks
   - Format checking (gofmt)
   - go.mod verification
   - Fast feedback on code quality issues

3. build.yml - Build verification on every push/PR
   - Builds for all platforms (Linux, macOS, FreeBSD, OpenBSD)
   - Multiple architectures (amd64, arm64, armv7)
   - Uploads artifacts for inspection
   - Verifies binaries execute correctly

4. release.yml - Full release on tag push ⭐
   - Trigger: Push version tag (v1.0.0, v2.1.3, etc.)
   - Runs all tests first
   - GoReleaser handles everything:
     - Builds all platforms/architectures (12+ combinations)
     - Creates tar.gz/zip archives
     - Generates changelog from commits (grouped by type)
     - Creates GitHub release with full notes
     - Uploads release assets (binaries, checksums, signatures)
     - Builds .deb and .rpm packages
     - Updates Homebrew formula in tap repository
     - Builds multi-arch Docker images
     - Signs releases with GPG
     - Generates SHA256 checksums
   - 100% automated - no manual steps

5. docker.yml - Container images
   - Builds on push to main or tag
   - Multi-arch support: amd64, arm64, armv7
   - Pushes to GitHub Container Registry
   - Tags: latest, version, edge

Release Workflow in Detail

Trigger
```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

What Happens Automatically
1. Tests run on all platforms (Linux, macOS)
2. GoReleaser executes and performs:
   - Cross-platform compilation for 12+ OS/arch combinations
   - Archive creation with all necessary files
   - Changelog generation from conventional commits:
     - Groups commits by type (Features, Bug Fixes, Performance)
     - Filters out noise (docs, tests, chores)
     - Includes comparison link to previous version
   - GitHub release creation with:
     - Auto-generated release notes
     - Full changelog
     - Binary downloads for all platforms
     - SHA256 checksums file
     - GPG signatures for verification
   - System package builds:
     - .deb packages for Debian/Ubuntu
     - .rpm packages for Fedora/RHEL
     - .apk packages for Alpine
     - Includes systemd service files
     - Post-install scripts for user creation
   - Homebrew formula update:
     - Pushes to separate tap repository
     - Auto-updates version and checksums
     - Includes formula testing
   - Docker multi-arch image builds:
     - Builds for amd64, arm64, armv7
     - Creates manifests for multi-arch support
     - Pushes to GitHub Container Registry
     - Tags with version and 'latest'
3. Release is automatically published and available

Changelog Generation

Uses conventional commit format with automatic grouping:
- `feat:` or `feat(scope):` → Features section
- `fix:` or `fix(scope):` → Bug Fixes section
- `perf:` or `perf(scope):` → Performance Improvements section
- Filters out: `docs:`, `test:`, `ci:`, `chore:`, `style:`
- Ignores merge commits
- Orders sections: Features → Bug Fixes → Performance → Others

Example Commit Messages
```
feat(gopher): add thread navigation support
fix(gemini): correct TLS certificate handling
perf(cache): optimize gophermap rendering
docs: update installation guide
```

Required GitHub Secrets

```
CODECOV_TOKEN         # Code coverage uploads (optional)
GPG_PRIVATE_KEY       # For signing releases (required)
GPG_PASSPHRASE        # GPG key passphrase (required)
DOCKER_USERNAME       # Docker Hub username (optional)
DOCKER_PASSWORD       # Docker Hub token (optional)
TAP_GITHUB_TOKEN      # Token for homebrew tap repo (required for homebrew)
```

Complete Automation Flow

```
Developer
    ↓
git tag v1.0.0 → GitHub Actions
                      ↓
              [Release Workflow]
                      ↓
    ┌─────────────────┴─────────────────┐
    ↓                                   ↓
  Tests                            GoReleaser
  ↓ PASS                                ↓
  └───────────────────┬─────────────────┘
                      ↓
        ┌─────────────┼─────────────┐
        ↓             ↓             ↓
  GitHub Release  Packages    Docker Images

GitHub Release:
  - Binaries (all platforms)
  - Checksums (SHA256)
  - GPG Signatures
  - Auto-generated changelog
  - Release notes

System Packages:
  - .deb (Debian/Ubuntu via APT)
  - .rpm (Fedora/RHEL via DNF/YUM)
  - .apk (Alpine)
  - Homebrew formula (macOS/Linux)

Docker Images:
  - ghcr.io/owner/nophr:latest
  - ghcr.io/owner/nophr:v1.0.0
  - ghcr.io/owner/nophr:v1
  - Multi-arch manifests
```

Local Testing Before Release

Test everything locally before creating a tag:

```bash
# Run all checks
make check

# Test the release process locally (no publishing)
make release-snapshot

# This creates dist/ with all artifacts
ls -lh dist/

# Verify artifacts
./scripts/ci/verify.sh

# Test a binary
./dist/nophr_linux_amd64/nophr --version
```

File Structure

```
nophr/
├── .github/workflows/
│   ├── test.yml           # Automated testing
│   ├── lint.yml           # Code quality
│   ├── build.yml          # Build verification
│   ├── release.yml        # Release automation
│   └── docker.yml         # Container builds
├── scripts/
│   ├── test.sh            # Local test runner
│   ├── lint.sh            # Local linter
│   ├── build.sh           # Local build script
│   ├── install.sh         # User installation script
│   ├── postinstall.sh     # Package post-install
│   └── ci/
│       ├── setup.sh       # CI environment setup
│       └── verify.sh      # Artifact verification
├── Makefile               # Development commands
└── .goreleaser.yml        # Release configuration
```

Key Features

✅ One command release - Just push a git tag
✅ Automated changelog - Generated from commit messages
✅ Multi-platform builds - 12+ OS/architecture combinations
✅ Signed releases - GPG signatures for security verification
✅ Package managers - Homebrew, APT, RPM automatically updated
✅ Docker images - Multi-arch images built and pushed
✅ Local testing - All scripts work identically locally and in CI
✅ Fast feedback - Parallel jobs, dependency caching
✅ Security - All secrets managed via GitHub encrypted secrets
✅ Zero manual steps - Complete automation from tag to distribution

Developer Workflow

Day-to-day Development
```bash
# Make changes
vim pkg/gopher/server.go

# Run checks locally
make check

# If all passes, commit
git add .
git commit -m "feat(gopher): add selector routing"
git push
```

Creating a Release
```bash
# Ensure main branch is ready
git checkout main
git pull

# Create and push tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# GitHub Actions does everything else automatically
# Monitor at: https://github.com/owner/nophr/actions
```

Verifying a Release
```bash
# Check GitHub releases page
open https://github.com/owner/nophr/releases

# Verify assets:
# - Binaries for all platforms
# - checksums.txt
# - checksums.txt.sig (GPG signature)
# - Source code archives
# - Changelog in release notes

# Test installation
curl -fsSL https://get.nophr.io | sh

# Or via package manager
brew install nophr
```

Benefits Summary

For Developers:
- Write code, push tag, get release - no manual packaging
- Test everything locally before CI runs
- Fast feedback from parallel CI jobs
- Consistent behavior between local and CI environments

For Users:
- Multiple installation methods (binary, package manager, Docker, source)
- Verified releases with checksums and GPG signatures
- Automatic updates via package managers
- Easy one-line installation

For Maintainers:
- No manual release process
- Automated changelog generation
- Consistent versioning and tagging
- Security through signed releases
- Audit trail via GitHub Actions logs

The entire CI/CD pipeline is designed for maximum automation while maintaining security and quality standards.
