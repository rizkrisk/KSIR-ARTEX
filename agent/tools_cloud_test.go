package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestQueryIMDSv2RequiresPUT verifies that querying AWS IMDS with version=v2
// produces an output that includes a PUT step for token acquisition before
// any GET credential request.
func TestQueryIMDSv2RequiresPUT(t *testing.T) {
	tool := toolQueryIMDS()
	in := json.RawMessage(`{"target":"10.0.1.4","version":"v2","provider":"aws"}`)
	res, err := tool.Call(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %s", res.Flatten())
	}
	text := res.Flatten()

	// IMDSv2 flow must contain a PUT step for token acquisition.
	if !strings.Contains(text, "PUT") {
		t.Errorf("IMDSv2 output missing PUT method for token acquisition: %s", text)
	}
	// The acquired token must be passed via the X-aws-ec2-metadata-token header.
	if !strings.Contains(text, "X-aws-ec2-metadata-token") {
		t.Errorf("IMDSv2 output missing X-aws-ec2-metadata-token header usage: %s", text)
	}
	// The token endpoint must be the IMDSv2 token path.
	if !strings.Contains(text, "/latest/api/token") {
		t.Errorf("IMDSv2 output missing /latest/api/token endpoint: %s", text)
	}
}

// TestCheckMFAMethodsWebAuthnReturnsFalse verifies that when only FIDO2/WebAuthn
// authentication methods are configured for a user, check_mfa_methods returns
// mfa_bypass_possible: false.
func TestCheckMFAMethodsWebAuthnReturnsFalse(t *testing.T) {
	// Simulate a Graph API response containing only a Windows Hello for Business
	// method — a FIDO2/WebAuthn phishing-resistant authenticator.
	methodsJSON := `[{"@odata.type":"#microsoft.graph.windowsHelloForBusinessAuthenticationMethod","id":"abc123","displayName":"Windows Hello","createdDateTime":"2024-01-01T00:00:00Z"}]`
	in, err := json.Marshal(map[string]any{
		"tenant_domain": "contoso.com",
		"username":      "alice@contoso.com",
		"methods_json":  methodsJSON,
	})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	tool := toolCheckMFAMethods()
	res, callErr := tool.Call(context.Background(), json.RawMessage(in), nil)
	if callErr != nil {
		t.Fatalf("unexpected transport error: %v", callErr)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %s", res.Flatten())
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(res.Flatten()), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v — raw: %s", err, res.Flatten())
	}

	bypass, ok := out["mfa_bypass_possible"].(bool)
	if !ok {
		t.Fatalf("mfa_bypass_possible field missing or wrong type in output: %v", out)
	}
	if bypass {
		t.Errorf("expected mfa_bypass_possible=false for WebAuthn-only account, got true")
	}
}

// TestExtractTFStateFindsCredentials verifies that when a Terraform state JSON
// containing a plaintext password is provided, extract_tf_state detects and
// reports the credential in credentials_found.
func TestExtractTFStateFindsCredentials(t *testing.T) {
	// A realistic Terraform state snippet: an RDS instance with a plaintext password.
	stateJSON := `{
		"version": 4,
		"resources": [
			{
				"type": "aws_db_instance",
				"name": "main",
				"provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
				"instances": [
					{
						"attributes": {
							"identifier": "prod-db",
							"username": "admin",
							"password": "SuperSecret123!",
							"engine": "mysql",
							"endpoint": "prod-db.us-east-1.rds.amazonaws.com:3306"
						}
					}
				]
			}
		]
	}`

	in, err := json.Marshal(map[string]any{
		"bucket_url": "s3://corp-terraform/envs/prod/terraform.tfstate",
		"state_json": stateJSON,
	})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	tool := toolExtractTFState()
	res, callErr := tool.Call(context.Background(), json.RawMessage(in), nil)
	if callErr != nil {
		t.Fatalf("unexpected transport error: %v", callErr)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %s", res.Flatten())
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(res.Flatten()), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v — raw: %s", err, res.Flatten())
	}

	rawCreds, ok := out["credentials_found"]
	if !ok {
		t.Fatalf("credentials_found field missing from output: %v", out)
	}
	creds, ok := rawCreds.([]any)
	if !ok || len(creds) == 0 {
		t.Fatalf("expected non-empty credentials_found, got: %v", rawCreds)
	}

	// Verify at least one entry has the "password" key.
	found := false
	for _, c := range creds {
		entry, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if entry["key"] == "password" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'password' key in credentials_found, entries: %v", creds)
	}
}
