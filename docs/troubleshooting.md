# Troubleshooting Guide

Common issues and solutions

Comprehensive troubleshooting guide for nophr: configuration errors, connection issues, database problems, protocol errors, and more.

## Quick Diagnostics

**Check if nophr is running:**
```bash
systemctl status nophr
# or
ps aux | grep nophr
```

**Check logs:**
```bash
# Systemd logs
journalctl -u nophr -n 100 -f

# Or if running manually
./dist/nophr --config nophr.yaml 2>&1 | tee nophr.log
```

**Check ports:**
```bash
sudo ss -tlnp | grep -E ':(70|79|1965)'
```

**Test protocols:**
```bash
# Gopher
echo "/" | nc localhost 70

# Finger
echo "" | nc localhost 79

# Gemini
echo "gemini://localhost/" | openssl s_client -connect localhost:1965 -quiet
```

---

## Configuration Errors

### "identity.npub is required"

**Error:**
```
Error loading configuration: invalid configuration: identity.npub is required
```

**Cause:** Missing `npub` in config.

**Fix:**
```yaml
identity:
  npub: "npub1abc..."  # Add your Nostr public key
```

Get your npub from any Nostr client.

---

### "identity.npub must start with 'npub1'"

**Error:**
```
invalid configuration: identity.npub must start with 'npub1'
```

**Cause:** Invalid npub format (might be hex pubkey instead).

**Fix:**
- Use `npub1...` format (bech32 encoding), not hex
- Convert hex → npub using Nostr client or online tool

---

### "at least one protocol must be enabled"

**Error:**
```
invalid configuration: at least one protocol must be enabled
```

**Cause:** All protocols disabled.

**Fix:** Enable at least one protocol:
```yaml
protocols:
  gopher:
    enabled: true  # Enable at least one
```

---

### "relay seed must start with ws:// or wss://"

**Error:**
```
invalid configuration: relay seed must start with ws:// or wss://: relay.damus.io
```

**Cause:** Missing protocol in relay URL.

**Fix:**
```yaml
relays:
  seeds:
    - "wss://relay.damus.io"  # Add wss:// prefix
```

---

### "invalid sync mode"

**Error:**
```
invalid configuration: invalid sync mode: friends (must be one of: self, following, mutual, foaf)
```

**Cause:** Typo in sync mode.

**Fix:**
```yaml
sync:
  scope:
    mode: "following"  # Use: self, following, mutual, or foaf
```

---

### "invalid storage driver"

**Error:**
```
invalid configuration: invalid storage driver: postgres (must be one of: sqlite, lmdb)
```

**Cause:** Unsupported storage driver.

**Fix:**
```yaml
storage:
  driver: "sqlite"  # Use: sqlite or lmdb
```

---

## Startup Errors

### "failed to initialize storage: unable to open database file"

**Error:**
```
failed to initialize storage: unable to open database file: /opt/nophr/data/nophr.db: no such file or directory
```

**Cause:** Data directory doesn't exist.

**Fix:**
```bash
mkdir -p /opt/nophr/data
# Ensure nophr user can write
sudo chown -R nophr:nophr /opt/nophr/data
```

---

### "failed to start Gopher server: listen tcp :70: bind: permission denied"

**Error:**
```
failed to start Gopher server: listen tcp :70: bind: permission denied
```

**Cause:** Ports <1024 require root/sudo.

**Fix:**

**Option 1: Systemd socket activation (recommended)**
See [Deployment Guide - Port Binding](deployment.md#port-binding)

**Option 2: Port forwarding**
```bash
# Forward port 70 → 7070
sudo iptables -t nat -A PREROUTING -p tcp --dport 70 -j REDIRECT --to-port 7070

# Change config
protocols:
  gopher:
    port: 7070
```

**Option 3: Run as root (NOT recommended)**
```bash
sudo /usr/local/bin/nophr --config /opt/nophr/nophr.yaml
```

---

### "failed to start Gopher server: listen tcp :70: bind: address already in use"

**Error:**
```
failed to start Gopher server: listen tcp :70: bind: address already in use
```

**Cause:** Another process is using port 70.

**Fix:**

**Find the process:**
```bash
sudo ss -tlnp | grep :70
# or
sudo lsof -i :70
```

**Kill the process or change nophr's port:**
```yaml
protocols:
  gopher:
    port: 7070  # Use different port
```

---

### "failed to initialize TLS: open ./certs/cert.pem: no such file or directory"

**Error:**
```
failed to create Gemini server: failed to initialize TLS: open ./certs/cert.pem: no such file or directory
```

**Cause:** TLS certificate file missing.

**Fix:**

**Option 1: Auto-generate (easiest)**
```yaml
protocols:
  gemini:
    tls:
      auto_generate: true
```

**Option 2: Create directory and let auto-generate work**
```bash
mkdir -p ./certs
```

**Option 3: Generate manually**
```bash
mkdir -p ./certs
openssl req -x509 -newkey rsa:4096 -keyout ./certs/key.pem \
  -out ./certs/cert.pem -days 365 -nodes \
  -subj "/CN=localhost"
```

---

## Protocol Connection Issues

### Gopher: "Connection refused"

**Symptom:**
```bash
$ telnet localhost 70
telnet: Unable to connect to remote host: Connection refused
```

**Causes:**
1. nophr not running
2. Gopher server disabled
3. Wrong port
4. Firewall blocking

**Fix:**

**Check if running:**
```bash
systemctl status nophr
```

**Check if enabled:**
```yaml
protocols:
  gopher:
    enabled: true  # Must be true
```

**Check port:**
```bash
sudo ss -tlnp | grep :70
```

**Check firewall:**
```bash
sudo ufw status
# Allow port if blocked
sudo ufw allow 70/tcp
```

---

### Gopher: "Empty response"

**Symptom:**
```bash
$ echo "/" | nc localhost 70
(no output, connection closes immediately)
```

**Causes:**
1. Database empty (no events)
2. Invalid selector
3. Rendering error

**Fix:**

**Check database:**
```bash
sqlite3 ./data/nophr.db "SELECT COUNT(*) FROM events;"
```

If 0 events, sync hasn't run or failed. Check logs:
```bash
journalctl -u nophr -n 100
```

**Try simple selector:**
```bash
echo "/" | nc localhost 70
```

Should return gophermap menu.

---

### Gemini: "Certificate verification failed"

**Symptom:**
```
Certificate verification failed: self signed certificate
```

**Cause:** Expected behavior with self-signed certificates.

**Fix:** Accept certificate in Gemini client (TOFU - Trust On First Use). This is normal for Gemini.

**For production, use Let's Encrypt:**
See [Deployment Guide - TLS Certificates](deployment.md#tls-certificates)

---

### Gemini: "No response / Timeout"

**Symptom:**
```bash
$ echo "gemini://localhost/" | openssl s_client -connect localhost:1965
(hangs, no response)
```

**Causes:**
1. nophr not listening
2. TLS handshake failed
3. Wrong port

**Fix:**

**Check listening:**
```bash
sudo ss -tlnp | grep :1965
```

**Test TLS handshake:**
```bash
openssl s_client -connect localhost:1965 -showcerts
```

Should show certificate details. If hangs, TLS config issue.

**Check logs:**
```bash
journalctl -u nophr | grep -i tls
```

---

### Finger: "Connection closed by remote host"

**Symptom:**
```bash
$ finger @localhost
Connection closed by remote host
```

**Causes:**
1. Query parsing error
2. No profile data (kind 0)
3. Finger server crashed

**Fix:**

**Check logs:**
```bash
journalctl -u nophr | grep -i finger
```

**Test with raw query:**
```bash
echo "" | nc localhost 79
```

**Check if profile exists:**
```bash
sqlite3 ./data/nophr.db "SELECT * FROM events WHERE kind = 0 LIMIT 1;"
```

If no kind 0, profile not synced yet.

---

## Database Issues

### "database is locked"

**Error:**
```
failed to query events: database is locked
```

**Cause:** Multiple processes accessing SQLite database, or unclean shutdown.

**Fix:**

**Ensure only one nophr instance:**
```bash
ps aux | grep nophr
# Kill duplicates
sudo systemctl restart nophr
```

**Check for stale locks:**
```bash
ls -la ./data/nophr.db*
# Remove WAL/SHM files if nophr is stopped
rm ./data/nophr.db-shm ./data/nophr.db-wal
```

**Database tuning (for frequent vacuum needs):**
```yaml
storage:
  # LMDB is not supported in this build; use SQLite and consider VACUUM/INDEX tuning
```

---

### "LMDB: database full"

Note: LMDB is not supported in this build. For now, use SQLite guidance above.

**Error:**
```
failed to store event: MDB_MAP_FULL: Environment mapsize limit reached
```

**Cause:** LMDB database reached `lmdb_max_size_mb` limit.

LMDB is not supported in this build, so you will not see this error with current binaries. The configuration keys are reserved for future LMDB support.

---

### "database disk image is malformed"

**Error:**
```
failed to query events: database disk image is malformed
```

**Cause:** Database corruption (power loss, disk error, bug).

**Fix:**

**Restore from backup:**
```bash
sudo systemctl stop nophr
cp /var/backups/nophr/nophr-20251024.db ./data/nophr.db
sudo systemctl start nophr
```

**Or delete and re-sync:**
```bash
sudo systemctl stop nophr
rm ./data/nophr.db
sudo systemctl start nophr
# nophr will create new database and sync from relays
```

**Check disk integrity:**
```bash
sudo fsck /dev/sda1  # Replace with your partition
```

---

## Sync Issues

### "No events syncing"

**Symptom:** Database remains empty, no events stored.

**Causes:**
1. Invalid npub
2. Unreachable seed relays
3. Sync scope too restrictive
4. No events matching filters

**Diagnose:**

**Check npub:**
```yaml
identity:
  npub: "npub1..."  # Verify this is YOUR npub
```

**Check seed relays:**
```bash
# Test manually
wscat -c wss://relay.damus.io
# Should connect without error
```

**Check sync scope:**
```yaml
sync:
  scope:
    mode: "self"  # Start with self, verify events exist
```

**Check logs:**
```bash
journalctl -u nophr | grep -i sync
journalctl -u nophr | grep -i relay
```

**Check relay hints:**
```bash
sqlite3 ./data/nophr.db "SELECT * FROM relay_hints LIMIT 10;"
```

If empty, relay discovery failed.

---

### "Sync is slow"

**Symptom:** Sync takes hours, events trickle in slowly.

**Causes:**
1. Too many authors (FOAF depth too high)
2. Slow/overloaded relays
3. Many concurrent subscriptions

**Fix:**

**Reduce scope:**
```yaml
sync:
  scope:
    mode: "following"  # Instead of foaf
    max_authors: 1000  # Lower cap
```

**Use faster relays:**
```yaml
relays:
  seeds:
    - "wss://relay.damus.io"  # Fast, well-connected
    - "wss://nos.lol"
```

**Increase concurrent subscriptions:**
```yaml
relays:
  policy:
    max_concurrent_subs: 16  # Increase from 8
```

---

### "Missing interactions (replies, reactions, zaps)"

**Symptom:** Events show 0 replies/reactions when they should have some.

**Causes:**
1. Kinds 7, 9735 not in sync.kinds
2. Inbox filters disabled
3. Aggregates not computed

**Fix:**

**Check sync kinds:**
```yaml
sync:
  kinds: [0, 1, 3, 6, 7, 9735, 30023, 10002]  # Include 7 and 9735
```

**Check inbox settings:**
```yaml
inbox:
  include_replies: true
  include_reactions: true
  include_zaps: true
```

**Check aggregates:**
```bash
sqlite3 ./data/nophr.db "SELECT * FROM aggregates LIMIT 10;"
```

If empty, aggregates not computed. Check logs:
```bash
journalctl -u nophr | grep -i aggregate
```

---

## Performance Issues

### "High CPU usage"

**Symptom:** nophr uses 50-100% CPU constantly.

**Causes:**
1. Sync engine running (expected during initial sync)
2. Aggregate reconciler running
3. Many concurrent connections
4. Markdown rendering (CPU-bound)

**Diagnose:**
```bash
top -p $(pgrep nophr)
```

**Fix:**

**If during initial sync:**
- Expected; wait for completion
- Check progress: `SELECT COUNT(*) FROM events;`

**If after sync complete:**
- Check if reconciler running too frequently
```yaml
caching:
  aggregates:
    reconciler_interval_seconds: 3600  # Increase from 900
```

- Reduce scope if syncing too many authors

---

### "High memory usage"

**Symptom:** nophr uses >1GB RAM.

**Causes:**
1. Large cache (if enabled)
2. Many events in memory (query results)

**Fix:**

**Monitor memory:**
```bash
ps aux | grep nophr
```

**Reduce query sizes:**
- Not user-configurable yet
- Future: add pagination limits

---

### "Slow page load"

**Symptom:** Protocol requests take seconds to respond.

**Causes:**
1. Caching disabled or misconfigured
2. Large database queries
3. Complex markdown rendering

**Diagnose:**

**Check query time:**
```bash
time sqlite3 ./data/nophr.db "SELECT * FROM events WHERE kind = 1 LIMIT 100;"
```

**Fix:**
- Enable and tune caching (see configuration caching settings)
- Current: optimize sync scope to reduce event count

---

## Logging and Debugging

### Enable debug logging

**In config:**
```yaml
logging:
  level: "debug"  # Change from info
```

**Or via environment:**
```bash
NOPHR_LOG_LEVEL=debug nophr --config nophr.yaml
```

**Restart:**
```bash
sudo systemctl restart nophr
```

**View debug logs:**
```bash
journalctl -u nophr -f
```

---

### Common log messages

**"[INFO] Starting nophr"**
- Normal startup message

**"[INFO] Initializing storage... Storage: sqlite initialized"**
- Storage layer initialized successfully

**"[WARN] Failed to connect to relay: wss://relay.example.com"**
- Relay unreachable; will retry with backoff

**"[ERROR] Failed to store event: ..."**
- Event storage failed; check database permissions/disk space

**"[DEBUG] Received event: kind=1 pubkey=..."**
- Event received from relay (debug mode)

---

## Getting Help

### Check existing documentation

- [Getting Started](getting-started.md)
- [Configuration Reference](configuration.md)
- [Storage Guide](storage.md)
- [Protocols Guide](protocols.md)
- [Nostr Integration](nostr-integration.md)
- [Architecture Overview](architecture.md)
- [Deployment Guide](deployment.md)

### Search issues

Check if issue already reported:
- GitHub Issues: https://github.com/sandwichfarm/nophr/issues

### Report new issue

Include:
- nophr version: `nophr --version`
- Operating system
- Configuration (remove nsec!)
- Relevant logs
- Steps to reproduce

### Community support

- GitHub Discussions: https://github.com/sandwichfarm/nophr/discussions
- Nostr: Contact operator (check README for contact info)

---

## Emergency Procedures

### nophr won't start

```bash
# Check logs
journalctl -u nophr -n 100

# Validate config
cat /opt/nophr/nophr.yaml | grep -E '(npub|driver|enabled)'

# Check permissions
ls -la /opt/nophr/
ls -la /opt/nophr/data/

# Test manually (as nophr user)
sudo -u nophr /usr/local/bin/nophr --config /opt/nophr/nophr.yaml
```

---

### Database corrupted

```bash
# Stop nophr
sudo systemctl stop nophr

# Backup current (corrupted) database
cp /opt/nophr/data/nophr.db /opt/nophr/data/nophr.db.corrupted

# Option 1: Restore from backup
cp /var/backups/nophr/nophr-20251024.db /opt/nophr/data/nophr.db

# Option 2: Start fresh (re-sync)
rm /opt/nophr/data/nophr.db

# Start nophr
sudo systemctl start nophr
```

---

### Out of disk space

```bash
# Check disk usage
df -h

# Check database size
du -h /opt/nophr/data/nophr.db

# Free space: reduce retention
sync:
  retention:
    keep_days: 90  # Reduce from 365

# Or manual prune
sqlite3 /opt/nophr/data/nophr.db "DELETE FROM events WHERE created_at < (strftime('%s', 'now') - 90*86400);"
sqlite3 /opt/nophr/data/nophr.db "VACUUM;"

# Restart
sudo systemctl restart nophr
```

---

### Complete reset

**Warning: Deletes all data!**

```bash
# Stop nophr
sudo systemctl stop nophr

# Backup config
cp /opt/nophr/nophr.yaml /opt/nophr/nophr.yaml.bak

# Delete data
rm -rf /opt/nophr/data/*

# Start nophr (will recreate database)
sudo systemctl start nophr
```

---

**Next:** [Getting Started](getting-started.md) | [Configuration Reference](configuration.md) | [Deployment Guide](deployment.md)
