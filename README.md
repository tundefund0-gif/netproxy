# netproxy

Fast single-binary HTTP + SOCKS5 proxy with built-in DNS cache. Zero dependencies. Works on phones (Termux), servers, Raspberry Pi — any device with a network stack.

## Quick start

### On Android (Termux)

```bash
pkg install golang git
go install github.com/tundefund0-gif/netproxy@latest
netproxy -http 8080 -socks 1080
```

### On Linux (any device)

```bash
# Download pre-built binary (no Go needed)
wget https://github.com/tundefund0-gif/netproxy/releases/latest/download/netproxy_latest_linux_arm64
chmod +x netproxy_latest_linux_arm64
./netproxy_latest_linux_arm64 -http 8080 -socks 1080
```

Or build from source:

```bash
git clone https://github.com/tundefund0-gif/netproxy
cd netproxy
go build -o netproxy .
./netproxy
```

## Usage

```bash
# Both proxies on default ports
./netproxy

# Custom ports
./netproxy -http 8080 -socks 1080

# With authentication (both HTTP + SOCKS5)
./netproxy -auth "myuser:mypass"

# Custom DNS
./netproxy -dns "8.8.8.8:53"

# Bind to specific interface
./netproxy -bind "192.168.1.10"
```

## Client configuration

### On the other phone / laptop

**WiFi proxy settings** (easiest):

| Setting | Value |
|---------|-------|
| Proxy type | SOCKS5 or HTTP |
| Host | IP of the device running netproxy |
| Port | 1080 (SOCKS5) or 8080 (HTTP) |

**Browser proxy** (Firefox):

Settings → Network → Proxy → Manual proxy configuration → SOCKS5 → IP:1080

**Command line**:

```bash
# HTTP proxy
export HTTP_PROXY=http://192.168.1.10:8080
export HTTPS_PROXY=http://192.168.1.10:8080

# SOCKS5
curl --socks5 192.168.1.10:1080 https://example.com
```

## Architecture

```
Client → SOCKS5/HTTP → netproxy → DNS cache (8192-slot hash table)
                                    ↓
                              Upstream DNS (1.1.1.1)
                                    ↓
                              Target server (TCP tunnel)
```

- HTTP proxy: GET/POST + CONNECT for HTTPS
- SOCKS5: CONNECT command with optional username/password auth (RFC 1929)
- DNS: open-addressing hash table, FNV-1a hashing, TTL-aware, pre-warms 7 domains
- Buffer pool: 32KB buffers recycled via sync.Pool, zero alloc on data path
- TCP: NODELAY enabled, keepalive 30s

## Performance

Tested on [environment]:

```
HTTP proxy:  450-500ms per request (includes full round-trip to target)
SOCKS5:      550-750ms per request
Proxy overhead: <1ms (cached DNS + zero-copy tunneling)
```

## Build for ARM

```bash
# ARM64 (modern phones, Raspberry Pi 3+)
GOOS=linux GOARCH=arm64 go build -o netproxy .

# ARMv7 (older phones, Raspberry Pi Zero)
GOOS=linux GOARCH=arm GOARM=7 go build -o netproxy .

# Build all platforms
./build.sh
```

## Download

Pre-built binaries for amd64, arm64, armv7: [Releases](https://github.com/tundefund0-gif/netproxy/releases)
