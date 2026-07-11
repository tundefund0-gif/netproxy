#!/bin/sh
set -e
PORT_HTTP=9990
PORT_SOCKS=9991

echo "=== netproxy benchmark ==="
echo ""

# Start proxy
./netproxy -http $PORT_HTTP -socks $PORT_SOCKS 2>/dev/null &
PID=$!
sleep 1

# Check if hey is installed
if command -v hey >/dev/null 2>&1; then
    HEY=hey
elif command -v wrk >/dev/null 2>&1; then
    HEY="wrk -t2 -c10"
else
    # Fallback to curl sequential timing
    echo "No benchmark tool found, using curl x10..."
    HEY=""
fi

if [ -n "$HEY" ]; then
    echo "1. HTTP proxy via localhost"
    $HEY -n 500 -c 10 -m GET -H "Host: example.com" http://127.0.0.1:$PORT_HTTP/http://example.com/ 2>&1 | tail -8
    echo ""

    echo "2. SOCKS5 proxy via localhost"
    $HEY -n 500 -c 10 -m GET http://127.0.0.1:$PORT_SOCKS/http://example.com/ 2>&1 | tail -8
    echo ""
fi

echo "3. Sequential latency test (10 requests)"
echo "   HTTP:"
for i in $(seq 1 10); do
    curl -s -o /dev/null -w "%{time_total} " --proxy http://127.0.0.1:$PORT_HTTP --max-time 5 http://example.com
done
echo ""

echo "   SOCKS5:"
for i in $(seq 1 10); do
    curl -s -o /dev/null -w "%{time_total} " --socks5 127.0.0.1:$PORT_SOCKS --max-time 5 http://example.com
done
echo ""

kill $PID 2>/dev/null; wait $PID 2>/dev/null
echo ""
echo "=== benchmark done ==="
