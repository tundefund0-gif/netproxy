# netproxy

Single-binary HTTP + SOCKS5 proxy with DNS cache. Runs on any device (phone, server, laptop). Connect other devices to it for proxied internet access.

## Usage

```bash
# Start both proxies
./netproxy

# Custom ports
./netproxy -http 8080 -socks 1080

# With auth
./netproxy -auth "user:pass"

# Custom DNS upstream
./netproxy -dns "8.8.8.8:53"

# Bind all interfaces (default)
./netproxy -bind "0.0.0.0"
```

## Client setup

### SOCKS5
```
Device → Settings → WiFi → Proxy → Manual
Host: <ip of proxy device>
Port: 1080
```

Or Firefox → Settings → Network → Proxy → SOCKS5

### HTTP
```
Device → Settings → WiFi → Proxy → Manual
Host: <ip of proxy device>
Port: 8080
```

## Build

```bash
git clone https://github.com/tundefund0-gif/netproxy
cd netproxy
go build -o netproxy .
```

## Features

- HTTP proxy (GET/POST/CONNECT for HTTPS)
- SOCKS5 with optional username/password auth
- Built-in DNS cache (pre-warms top 5 domains)
- Single binary, zero dependencies
