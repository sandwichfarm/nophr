# Deployment Guide

Production deployment guide

Complete guide to deploying nophr in production: system configuration, port binding, TLS certificates, systemd services, and monitoring.

## Quick Install

The fastest way to deploy nophr:

```bash
# Download and run installer
curl -sSL https://nophr.io/install.sh | sh

# The installer will:
# - Download the latest release for your platform
# - Install to /usr/local/bin/nophr
# - Create example configuration
```

**For detailed installation options** (Docker, packages, building from source), see [installation.md](installation.md).

## Prerequisites

- Linux server (Ubuntu 20.04+, Debian 11+, or similar)
- Root/sudo access
- Public IP or domain name
- Open ports: 70 (Gopher), 79 (Finger), 1965 (Gemini)

---

## Table of Contents

- [Quick Install](#quick-install)
- [System Requirements](#system-requirements)
- [Port Binding](#port-binding)
- [TLS Certificates](#tls-certificates)
- [Systemd Service](#systemd-service)
- [Reverse Proxy](#reverse-proxy)
- [Docker Deployment](#docker-deployment)
- [Redis Setup](#redis-setup)
- [Firewall](#firewall)
- [Monitoring](#monitoring)
- [Backups](#backups)
- [Updates](#updates)

---

## System Requirements

### Minimum

- **CPU:** 1 core
- **RAM:** 256MB
- **Disk:** 1GB (for binary + database)
- **Network:** Stable internet, low latency to Nostr relays

### Recommended

- **CPU:** 2 cores
- **RAM:** 512MB-1GB
- **Disk:** 5-10GB (for database growth)
- **Network:** <100ms latency to major Nostr relays

### Scaling

**For <100K events:**
- SQLite backend
- 512MB RAM
- 1 core

**For >100K events:**
- LMDB backend
- 1GB RAM
- 2 cores

---

## Port Binding

Ports below 1024 require root privileges. nophr needs ports 70, 79, and optionally 1965 (usually OK).

### Option 1: systemd Socket Activation (Recommended)

Systemd can bind ports as root, then pass sockets to unprivileged nophr process.

**Create socket units:**

`/etc/systemd/system/nophr-gopher.socket`:
```ini
[Unit]
Description=nophr Gopher Socket
PartOf=nophr.service

[Socket]
ListenStream=0.0.0.0:70
Accept=no

[Install]
WantedBy=sockets.target
```

`/etc/systemd/system/nophr-finger.socket`:
```ini
[Unit]
Description=nophr Finger Socket
PartOf=nophr.service

[Socket]
ListenStream=0.0.0.0:79
Accept=no

[Install]
WantedBy=sockets.target
```

`/etc/systemd/system/nophr-gemini.socket`:
```ini
[Unit]
Description=nophr Gemini Socket
PartOf=nophr.service

[Socket]
ListenStream=0.0.0.0:1965
Accept=no

[Install]
WantedBy=sockets.target
```

**Update service unit:**

`/etc/systemd/system/nophr.service` (add `Requires` and `After`):
```ini
[Unit]
Description=nophr - Nostr to Gopher/Gemini/Finger Gateway
After=network.target nophr-gopher.socket nophr-finger.socket nophr-gemini.socket
Requires=nophr-gopher.socket nophr-finger.socket nophr-gemini.socket

[Service]
Type=simple
User=nophr
Group=nophr
WorkingDirectory=/opt/nophr
ExecStart=/usr/local/bin/nophr --config /opt/nophr/nophr.yaml
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
```

**Enable and start:**
```bash
sudo systemctl enable nophr-gopher.socket nophr-finger.socket nophr-gemini.socket
sudo systemctl start nophr-gopher.socket nophr-finger.socket nophr-gemini.socket
sudo systemctl enable nophr.service
sudo systemctl start nophr.service
```

**Verify:**
```bash
sudo systemctl status nophr
sudo ss -tlnp | grep -E ':(70|79|1965)'
```

### Option 2: Port Forwarding (iptables)

Forward high ports → low ports, run nophr on high ports.

**Configure nophr on high ports:**
```yaml
protocols:
  gopher:
    port: 7070
  finger:
    port: 7979
  gemini:
    port: 11965
```

**Forward ports:**
```bash
# Gopher: 70 → 7070
sudo iptables -t nat -A PREROUTING -p tcp --dport 70 -j REDIRECT --to-port 7070

# Finger: 79 → 7979
sudo iptables -t nat -A PREROUTING -p tcp --dport 79 -j REDIRECT --to-port 7979

# Gemini: 1965 → 11965
sudo iptables -t nat -A PREROUTING -p tcp --dport 1965 -j REDIRECT --to-port 11965

# Save rules
sudo iptables-save > /etc/iptables/rules.v4
```

**Make persistent:**
```bash
# Install iptables-persistent
sudo apt install iptables-persistent

# Or on systemd-based systems, create service
sudo systemctl enable netfilter-persistent
sudo systemctl start netfilter-persistent
```

### Option 3: Run as Root (Not Recommended)

**Only for testing!** Do not run production as root.

```bash
sudo /usr/local/bin/nophr --config /opt/nophr/nophr.yaml
```

---

## TLS Certificates

Gemini requires TLS. Two options: self-signed (personal use) or Let's Encrypt (production).

### Self-Signed Certificates

**Automatic (recommended):**

```yaml
protocols:
  gemini:
    tls:
      auto_generate: true
```

nophr generates self-signed cert on first run.

**Manual generation:**
```bash
mkdir -p /opt/nophr/certs
openssl req -x509 -newkey rsa:4096 \
  -keyout /opt/nophr/certs/key.pem \
  -out /opt/nophr/certs/cert.pem \
  -days 365 -nodes \
  -subj "/CN=gemini.example.com"

chown nophr:nophr /opt/nophr/certs/*.pem
chmod 600 /opt/nophr/certs/key.pem
```

**Configuration:**
```yaml
protocols:
  gemini:
    tls:
      cert_path: "/opt/nophr/certs/cert.pem"
      key_path: "/opt/nophr/certs/key.pem"
      auto_generate: false
```

**Note:** Self-signed certs require TOFU (Trust On First Use) in Gemini clients.

### Let's Encrypt Certificates

**Using certbot:**

```bash
# Install certbot
sudo apt install certbot

# Get certificate (HTTP-01 challenge)
# Note: Requires port 80 open temporarily
sudo certbot certonly --standalone -d gemini.example.com

# Certificates saved to:
# /etc/letsencrypt/live/gemini.example.com/fullchain.pem
# /etc/letsencrypt/live/gemini.example.com/privkey.pem
```

**Copy to nophr:**
```bash
sudo cp /etc/letsencrypt/live/gemini.example.com/fullchain.pem /opt/nophr/certs/cert.pem
sudo cp /etc/letsencrypt/live/gemini.example.com/privkey.pem /opt/nophr/certs/key.pem
sudo chown nophr:nophr /opt/nophr/certs/*.pem
sudo chmod 600 /opt/nophr/certs/key.pem
```

**Configuration:**
```yaml
protocols:
  gemini:
    tls:
      cert_path: "/opt/nophr/certs/cert.pem"
      key_path: "/opt/nophr/certs/key.pem"
      auto_generate: false
```

**Auto-renewal:**

Certbot creates cron job automatically. Add post-renewal hook:

`/etc/letsencrypt/renewal-hooks/post/nophr-reload.sh`:
```bash
#!/bin/bash
cp /etc/letsencrypt/live/gemini.example.com/fullchain.pem /opt/nophr/certs/cert.pem
cp /etc/letsencrypt/live/gemini.example.com/privkey.pem /opt/nophr/certs/key.pem
chown nophr:nophr /opt/nophr/certs/*.pem
chmod 600 /opt/nophr/certs/key.pem
systemctl restart nophr
```

```bash
sudo chmod +x /etc/letsencrypt/renewal-hooks/post/nophr-reload.sh
```

---

## Systemd Service

Run nophr as a systemd service for automatic startup and management.

### Create User

```bash
sudo useradd --system --create-home --home-dir /opt/nophr --shell /bin/bash nophr
```

### Install Binary

```bash
sudo cp dist/nophr /usr/local/bin/nophr
sudo chmod +x /usr/local/bin/nophr
```

### Create Configuration

```bash
sudo mkdir -p /opt/nophr/data
sudo mkdir -p /opt/nophr/certs
sudo cp nophr.yaml /opt/nophr/nophr.yaml
sudo chown -R nophr:nophr /opt/nophr
```

### Create Service Unit

`/etc/systemd/system/nophr.service`:
```ini
[Unit]
Description=nophr - Nostr to Gopher/Gemini/Finger Gateway
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=nophr
Group=nophr
WorkingDirectory=/opt/nophr

# Main command
ExecStart=/usr/local/bin/nophr --config /opt/nophr/nophr.yaml

# Restart policy
Restart=on-failure
RestartSec=10s

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/nophr

# Resource limits
LimitNOFILE=65536

# Environment
Environment="NOPHR_NSEC_FILE=/opt/nophr/nsec"

[Install]
WantedBy=multi-user.target
```

**Store nsec securely:**
```bash
echo "nsec1..." | sudo tee /opt/nophr/nsec
sudo chmod 600 /opt/nophr/nsec
sudo chown nophr:nophr /opt/nophr/nsec
```

### Enable and Start

```bash
sudo systemctl daemon-reload
sudo systemctl enable nophr.service
sudo systemctl start nophr.service
```

### Check Status

```bash
sudo systemctl status nophr
sudo journalctl -u nophr -f
```

### Manage Service

```bash
# Start
sudo systemctl start nophr

# Stop
sudo systemctl stop nophr

# Restart
sudo systemctl restart nophr

# Reload config (if supported)
sudo systemctl reload nophr

# View logs
sudo journalctl -u nophr -n 100
```

---

## Reverse Proxy

For advanced setups, you can run nophr behind a reverse proxy (though protocols are non-HTTP).

### Gopher Proxy (socat)

Forward Gopher through socat:

```bash
sudo apt install socat

# Forward external :70 → nophr :7070
socat TCP4-LISTEN:70,fork TCP4:localhost:7070
```

**As systemd service:**

`/etc/systemd/system/gopher-proxy.service`:
```ini
[Unit]
Description=Gopher Proxy
After=network.target

[Service]
Type=simple
ExecStart=/usr/bin/socat TCP4-LISTEN:70,fork TCP4:localhost:7070
Restart=always

[Install]
WantedBy=multi-user.target
```

### Nginx Reverse Proxy

Example nginx configuration for Gemini TLS termination.

**Install nginx:**
```bash
sudo apt install nginx
```

**Configuration:** `/etc/nginx/nginx.conf` or `/etc/nginx/conf.d/nophr.conf`

```nginx
# Gemini protocol (port 1965 with TLS)
stream {
    upstream gemini_backend {
        server localhost:11965;
    }

    server {
        listen 1965 ssl;

        ssl_certificate /etc/ssl/certs/nophr.crt;
        ssl_certificate_key /etc/ssl/private/nophr.key;
        ssl_protocols TLSv1.2 TLSv1.3;
        ssl_ciphers HIGH:!aNULL:!MD5;

        proxy_pass gemini_backend;
        proxy_ssl off;
    }
}

# Optional: HTTP endpoint for monitoring/health checks
http {
    server {
        listen 8080;
        server_name _;

        location /health {
            access_log off;
            return 200 "OK\n";
            add_header Content-Type text/plain;
        }
    }
}
```

**See also:** `examples/nginx.conf` in the repository.

### Caddy Reverse Proxy

Caddy automatically handles TLS certificates via Let's Encrypt.

**Install Caddy:**
```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy
```

**Configuration:** `/etc/caddy/Caddyfile`

```
# Gemini protocol with automatic HTTPS
gemini.example.com:1965 {
    reverse_proxy localhost:11965
    tls {
        protocols tls1.2 tls1.3
    }
}

# Optional: Web-based status/monitoring page
status.example.com {
    reverse_proxy localhost:8080
}
```

**Restart Caddy:**
```bash
sudo systemctl restart caddy
```

**See also:** `examples/Caddyfile` in the repository.

---

## Docker Deployment

Deploy nophr using Docker and Docker Compose.

### Using Docker Compose (Recommended)

nophr includes a production-ready `docker-compose.yml`:

```bash
# Clone repository
git clone https://github.com/sandwichfarm/nophr.git
cd nophr

# Copy and edit configuration
cp configs/nophr.example.yaml configs/nophr.yaml
nano configs/nophr.yaml

# Set environment variables
export NOPHR_NSEC="nsec1..."  # Never commit this!
export NOPHR_LOG_LEVEL="info"

# Start services
docker-compose up -d

# View logs
docker-compose logs -f nophr

# Stop services
docker-compose down
```

### Docker Compose Features

The included `docker-compose.yml` provides:

**Main service:**
- nophr server with all three protocols
- Persistent volumes for data and certs
- Health checks
- Security hardening (no-new-privileges, minimal capabilities)
- Environment variable configuration

**Optional services** (uncomment to enable):

**Redis cache:**
```yaml
redis:
  image: redis:7-alpine
  command: redis-server --appendonly yes --maxmemory 512mb
```

**Caddy reverse proxy:**
```yaml
caddy:
  image: caddy:2-alpine
  ports:
    - "443:443"
  volumes:
    - ./examples/Caddyfile:/etc/caddy/Caddyfile:ro
```

### Docker Compose Configuration

**Environment variables:**

Create `.env` file:
```bash
NOPHR_NSEC=nsec1...
NOPHR_REDIS_URL=redis://redis:6379
NOPHR_LOG_LEVEL=info
```

**Volumes:**
- `nophr-data` - Database and sync state (persistent)
- `nophr-certs` - TLS certificates (persistent)
- `nophr-logs` - Application logs (optional)

### Standalone Docker

Run nophr directly with Docker:

```bash
# Pull image (when available)
docker pull ghcr.io/sandwichfarm/nophr:latest

# Or build locally
docker build -t nophr:latest .

# Run container
docker run -d \
  --name nophr \
  -p 70:70 \
  -p 79:79 \
  -p 1965:1965 \
  -v ./nophr.yaml:/etc/nophr/nophr.yaml:ro \
  -v nophr-data:/var/lib/nophr \
  -e NOPHR_NSEC="nsec1..." \
  ghcr.io/sandwichfarm/nophr:latest
```

### Docker Security

The Docker deployment includes security hardening:

```yaml
security_opt:
  - no-new-privileges:true  # Prevent privilege escalation
cap_drop:
  - ALL                      # Drop all capabilities
cap_add:
  - NET_BIND_SERVICE         # Only allow binding to ports
read_only: true              # Read-only filesystem
tmpfs:
  - /tmp                     # Writable /tmp
```

### Multi-Architecture Support

Docker images support multiple architectures:
- `amd64` (x86_64)
- `arm64` (ARM 64-bit)
- `arm/v7` (ARM 32-bit)

---

## Redis Setup

Redis is an optional cache backend that provides distributed caching, persistence across restarts, and better memory management for production deployments.

### When to Use Redis

**Use Redis cache when:**
- Running multiple nophr instances (load balancing)
- Need persistent cache across restarts
- Limited memory on host machine
- Want shared cache for distributed deployment

**Use memory cache when:**
- Single nophr instance
- Development/testing
- Simplicity preferred
- No external dependencies wanted

### Installation

**Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install redis-server
```

**RHEL/CentOS/Fedora:**
```bash
sudo dnf install redis
```

**macOS:**
```bash
brew install redis
```

**From Docker:**
```bash
docker run -d --name redis \
  -p 6379:6379 \
  -v redis-data:/data \
  redis:7-alpine \
  redis-server --appendonly yes --maxmemory 512mb
```

### Configuration

**Edit Redis config** (usually `/etc/redis/redis.conf`):

```conf
# Bind to localhost (or specific IP for remote access)
bind 127.0.0.1

# Port
port 6379

# Enable persistence (AOF)
appendonly yes
appendfsync everysec

# Memory limit
maxmemory 512mb
maxmemory-policy allkeys-lru

# Disable snapshotting (AOF is enough)
save ""

# Security: Set password (recommended)
requirepass your_redis_password_here
```

**Restart Redis:**
```bash
sudo systemctl restart redis
sudo systemctl enable redis
```

### nophr Configuration

**With Redis on localhost (no password):**
```yaml
caching:
  enabled: true
  engine: "redis"
  redis_url: "redis://localhost:6379/0"
```

**With Redis password:**
```yaml
caching:
  enabled: true
  engine: "redis"
  # Set via environment variable for security
```

```bash
export NOPHR_REDIS_URL="redis://:your_password@localhost:6379/0"
```

**Remote Redis server:**
```bash
export NOPHR_REDIS_URL="redis://:password@redis.example.com:6379/0"
```

### Redis Security

**Best practices:**

1. **Set a strong password:**
```conf
requirepass $(openssl rand -base64 32)
```

2. **Bind to specific interfaces:**
```conf
# Local only
bind 127.0.0.1

# Or specific IP
bind 192.168.1.10
```

3. **Disable dangerous commands:**
```conf
rename-command FLUSHDB ""
rename-command FLUSHALL ""
rename-command KEYS ""
rename-command CONFIG "CONFIG_$(openssl rand -hex 8)"
```

4. **Enable TLS** (Redis 6+):
```conf
tls-port 6380
port 0
tls-cert-file /path/to/redis.crt
tls-key-file /path/to/redis.key
tls-ca-cert-file /path/to/ca.crt
```

```bash
export NOPHR_REDIS_URL="rediss://:password@redis.example.com:6380/0"
```

### Monitoring Redis

**Check Redis status:**
```bash
redis-cli ping
# Should return: PONG
```

**Get Redis info:**
```bash
redis-cli info
```

**Monitor cache operations:**
```bash
redis-cli monitor
```

**Check memory usage:**
```bash
redis-cli INFO memory
```

**Check connected clients:**
```bash
redis-cli CLIENT LIST
```

### Redis for Multiple nophr Instances

When running multiple nophr instances behind a load balancer:

**Redis server** (shared):
```bash
# On redis.example.com
sudo apt install redis-server
sudo systemctl enable --now redis
```

**nophr instances** (all pointing to same Redis):
```bash
# Instance 1
export NOPHR_REDIS_URL="redis://:password@redis.example.com:6379/0"
./nophr --config nophr.yaml

# Instance 2 (same Redis URL)
export NOPHR_REDIS_URL="redis://:password@redis.example.com:6379/0"
./nophr --config nophr.yaml
```

**Benefits:**
- Cache shared across all instances
- Consistent responses from any instance
- Better cache hit rates
- Reduced database load

### Troubleshooting Redis

**Connection refused:**
```bash
# Check Redis is running
sudo systemctl status redis

# Check Redis is listening
sudo netstat -tlnp | grep 6379

# Check firewall
sudo ufw allow 6379
```

**Authentication failed:**
```bash
# Test Redis password
redis-cli -a your_password ping

# Verify NOPHR_REDIS_URL includes password
echo $NOPHR_REDIS_URL
```

**Memory issues:**
```bash
# Check memory usage
redis-cli INFO memory | grep used_memory_human

# Increase maxmemory if needed
redis-cli CONFIG SET maxmemory 1gb
```

**Slow performance:**
```bash
# Check slow queries
redis-cli SLOWLOG get 10

# Monitor operations
redis-cli monitor

# Check latency
redis-cli --latency
```

### Cache Performance Metrics

Monitor these metrics to ensure Redis is performing well:

**nophr cache stats:**
```
Cache Statistics:
  Hits: 9,500
  Misses: 500
  Hit Rate: 95%
  Avg Get Time: 2.3ms (Redis)
  Avg Set Time: 3.1ms (Redis)
```

**Target metrics:**
- Hit rate: > 80%
- Get time: < 5ms
- Set time: < 10ms

**Redis memory usage:**
```bash
redis-cli INFO stats | grep keyspace
# db0:keys=1234,expires=567,avg_ttl=298000
```

---

## Firewall

Ensure required ports are open.

### ufw (Ubuntu/Debian)

```bash
# Enable firewall
sudo ufw enable

# SSH (important!)
sudo ufw allow 22/tcp

# nophr ports
sudo ufw allow 70/tcp    # Gopher
sudo ufw allow 79/tcp    # Finger
sudo ufw allow 1965/tcp  # Gemini

# Check status
sudo ufw status
```

### iptables

```bash
# Allow SSH
sudo iptables -A INPUT -p tcp --dport 22 -j ACCEPT

# Allow nophr
sudo iptables -A INPUT -p tcp --dport 70 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 79 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 1965 -j ACCEPT

# Drop others
sudo iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
sudo iptables -A INPUT -j DROP

# Save
sudo iptables-save > /etc/iptables/rules.v4
```

### firewalld (RHEL/CentOS)

```bash
sudo firewall-cmd --permanent --add-port=70/tcp
sudo firewall-cmd --permanent --add-port=79/tcp
sudo firewall-cmd --permanent --add-port=1965/tcp
sudo firewall-cmd --reload
```

---

## Monitoring

### System Logs

**View logs:**
```bash
sudo journalctl -u nophr -f
```

**Recent errors:**
```bash
sudo journalctl -u nophr -p err -n 50
```

### Health Checks

**Check if nophr is running:**
```bash
sudo systemctl is-active nophr
```

**Check ports:**
```bash
sudo ss -tlnp | grep -E ':(70|79|1965)'
```

**Expected output:**
```
LISTEN  0  128  0.0.0.0:70    0.0.0.0:*  users:(("nophr",pid=1234,fd=3))
LISTEN  0  128  0.0.0.0:79    0.0.0.0:*  users:(("nophr",pid=1234,fd=4))
LISTEN  0  128  0.0.0.0:1965  0.0.0.0:*  users:(("nophr",pid=1234,fd=5))
```

### Protocol Tests

**Gopher:**
```bash
echo "/" | nc localhost 70
```

**Gemini:**
```bash
echo "gemini://localhost/" | openssl s_client -connect localhost:1965 -quiet
```

**Finger:**
```bash
echo "" | nc localhost 79
```

### Database Size

```bash
du -h /opt/nophr/data/nophr.db
```

### Event Count

```bash
sqlite3 /opt/nophr/data/nophr.db "SELECT COUNT(*) FROM events;"
```

### Automated Monitoring (cron)

**Create monitoring script:**

`/opt/nophr/scripts/health-check.sh`:
```bash
#!/bin/bash
# Check if nophr is running
if ! systemctl is-active --quiet nophr; then
    echo "nophr is down! Restarting..."
    systemctl restart nophr
    echo "nophr restarted at $(date)" >> /var/log/nophr-health.log
fi

# Check database size
DB_SIZE=$(du -m /opt/nophr/data/nophr.db | cut -f1)
if [ "$DB_SIZE" -gt 10000 ]; then
    echo "Database size exceeded 10GB: ${DB_SIZE}MB" >> /var/log/nophr-health.log
fi
```

**Add to cron:**
```bash
sudo chmod +x /opt/nophr/scripts/health-check.sh
sudo crontab -e
# Add line:
*/5 * * * * /opt/nophr/scripts/health-check.sh
```

---

## Backups

### Automated Backups

**Backup script:**

`/opt/nophr/scripts/backup.sh`:
```bash
#!/bin/bash
BACKUP_DIR="/var/backups/nophr"
DATE=$(date +%Y%m%d-%H%M%S)

mkdir -p "$BACKUP_DIR"

# Backup database
cp /opt/nophr/data/nophr.db "$BACKUP_DIR/nophr-$DATE.db"

# Backup config
cp /opt/nophr/nophr.yaml "$BACKUP_DIR/nophr-$DATE.yaml"

# Keep last 7 days
find "$BACKUP_DIR" -name "nophr-*.db" -mtime +7 -delete

echo "Backup completed: $DATE"
```

**Schedule daily backups:**
```bash
sudo chmod +x /opt/nophr/scripts/backup.sh
sudo crontab -e
# Add:
0 2 * * * /opt/nophr/scripts/backup.sh >> /var/log/nophr-backup.log 2>&1
```

### Off-site Backups

**rsync to remote server:**
```bash
rsync -avz /var/backups/nophr/ user@backup-server:/backups/nophr/
```

**Or use cloud storage (rclone):**
```bash
sudo apt install rclone
rclone sync /var/backups/nophr/ remote:nophr-backups/
```

---

## Updates

### Update nophr

**Stop service:**
```bash
sudo systemctl stop nophr
```

**Backup current binary:**
```bash
sudo cp /usr/local/bin/nophr /usr/local/bin/nophr.bak
```

**Install new binary:**
```bash
sudo cp dist/nophr /usr/local/bin/nophr
sudo chmod +x /usr/local/bin/nophr
```

**Start service:**
```bash
sudo systemctl start nophr
```

**Verify:**
```bash
sudo systemctl status nophr
/usr/local/bin/nophr --version
```

### Rollback

If update causes issues:

```bash
sudo systemctl stop nophr
sudo cp /usr/local/bin/nophr.bak /usr/local/bin/nophr
sudo systemctl start nophr
```

---

## Performance Tuning

### File Descriptors

Increase file descriptor limit for high traffic:

`/etc/security/limits.conf`:
```
nophr soft nofile 65536
nophr hard nofile 65536
```

Or in systemd service:
```ini
[Service]
LimitNOFILE=65536
```

### Database Tuning

**SQLite WAL mode** (already enabled by Khatru):
```bash
sqlite3 /opt/nophr/data/nophr.db "PRAGMA journal_mode=WAL;"
```

**Vacuum periodically:**
```bash
# Weekly cron
0 3 * * 0 sqlite3 /opt/nophr/data/nophr.db "VACUUM;"
```

### LMDB Max Size (future)

LMDB is not supported in this build. When LMDB support is added, high-volume deployments will need to set `lmdb_max_size_mb` appropriately, for example:

```yaml
storage:
  driver: "lmdb"
  lmdb_max_size_mb: 20480  # 20GB
```

---

## Troubleshooting Deployment

### "Permission denied" binding to port

**Cause:** Ports <1024 require root.

**Fix:** Use systemd socket activation or port forwarding.

### "Address already in use"

**Cause:** Another service using port.

**Fix:**
```bash
# Find process
sudo ss -tlnp | grep :70

# Kill process or change port
```

### "TLS handshake failed" (Gemini)

**Cause:** Invalid certificate or wrong path.

**Fix:**
- Check cert paths in config
- Verify cert files exist and have correct permissions
- Regenerate certificate

### Database locked

**Cause:** Multiple nophr instances or unclean shutdown.

**Fix:**
```bash
# Ensure only one instance
sudo systemctl stop nophr
# Check for stale locks
rm -f /opt/nophr/data/nophr.db-shm /opt/nophr/data/nophr.db-wal
sudo systemctl start nophr
```

### Service crashes on start

**Check logs:**
```bash
sudo journalctl -u nophr -n 100
```

**Common causes:**
- Invalid config (missing npub, bad relay URLs)
- Missing data directory
- Permission issues

---

## Security Checklist

- [ ] Run as non-root user (`nophr`)
- [ ] Use systemd socket activation or port forwarding
- [ ] Store nsec in separate file with 600 permissions
- [ ] Enable firewall (ufw/iptables)
- [ ] Use proper TLS certs (Let's Encrypt)
- [ ] Set up automated backups
- [ ] Monitor logs regularly
- [ ] Keep nophr updated
- [ ] Limit database size (retention policy)

---

**Next:** [Troubleshooting Guide](troubleshooting.md) | [Configuration Reference](configuration.md) | [Getting Started](getting-started.md)
