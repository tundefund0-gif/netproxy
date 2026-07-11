# netproxy

Single-binary HTTP + SOCKS5 proxy with DNS cache. Zero deps. Run on one device, connect others to it.

## Install on Termux (Android) — quickest

```bash
pkg install wget
wget https://github.com/tundefund0-gif/netproxy/releases/download/v1.0.0/netproxy_arm64
chmod +x netproxy_arm64
mv netproxy_arm64 $PREFIX/bin/netproxy
netproxy -http 8080 -socks 1080
```

For 32-bit ARM (older phones): replace `arm64` with `armv7`.

## Install via Go (any device)

```bash
go install github.com/tundefund0-gif/netproxy@latest
export PATH=$PATH:$(go env GOPATH)/bin
netproxy -http 8080 -socks 1080
```

## Download binary directly

```bash
# amd64
wget -O netproxy https://github.com/tundefund0-gif/netproxy/releases/download/v1.0.0/netproxy_amd64

# arm64 (modern phones, Raspberry Pi 3+)
wget -O netproxy https://github.com/tundefund0-gif/netproxy/releases/download/v1.0.0/netproxy_arm64

# armv7 (older phones)
wget -O netproxy https://github.com/tundefund0-gif/netproxy/releases/download/v1.0.0/netproxy_armv7

chmod +x netproxy
./netproxy
```

## Usage

```bash
netproxy                    # HTTP :8080 + SOCKS5 :1080
netproxy -auth user:pass    # with auth
netproxy -bind 0.0.0.0      # listen on all interfaces (default)
```

## Connect other devices

On the other phone/laptop → WiFi settings → Proxy → Manual

| Field | Value |
|-------|-------|
| Host | IP of device running netproxy |
| Port | 1080 (SOCKS5) or 8080 (HTTP) |

## Build

```bash
git clone https://github.com/tundefund0-gif/netproxy
cd netproxy && go build -o netproxy .
```

## Tech

- HTTP proxy (GET/POST/CONNECT), SOCKS5 (CONNECT)
- Optional auth: HTTP Basic + SOCKS5 username/password
- DNS: 8192-slot open-addressing cache, FNV-1a, TTL, prewarms 7 domains
- 32KB buffer pool recycled via sync.Pool
- TCP_NODELAY + keepalive on all connections
- Zero external dependencies
