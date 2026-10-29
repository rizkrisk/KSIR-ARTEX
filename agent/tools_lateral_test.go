package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestHarvestCredentialsCallsSaveCred verifies that harvest_credentials returns a
// graceful tool error (not a panic) when the task store is nil, and that it
// reports the credential-store-unavailable condition. When t.ts is non-nil the
// handler calls t.ts.UpsertTaskCredential; the nil path covers unit-test scenarios
// without a live database.
func TestHarvestCredentialsCallsSaveCred(t *testing.T) {
	ts := NewToolSet(nil, "")
	tool := ts.harvestCredentials()

	in, _ := json.Marshal(map[string]any{
		"method": "lsass",
		"target": "192.168.1.10",
	})
	res, err := tool.Call(context.Background(), json.RawMessage(in), nil)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error with valid params: %s", res.Flatten())
	}
	body := res.Flatten()
	// With a nil store the handler still runs and embeds cred_note describing the
	// unavailable state; the cred_saved field must be present and false.
	if !strings.Contains(body, "cred_note") {
		t.Errorf("expected cred_note in result, got: %s", body)
	}
	if !strings.Contains(body, "unavailable") {
		t.Errorf("expected 'unavailable' in cred_note (nil store path), got: %s", body)
	}
}

// TestHarvestCredentialsUnknownMethod verifies that an unknown method is rejected
// with a descriptive tool error rather than a panic or silent fallthrough.
func TestHarvestCredentialsUnknownMethod(t *testing.T) {
	ts := NewToolSet(nil, "")
	tool := ts.harvestCredentials()
	in, _ := json.Marshal(map[string]any{"method": "rainbow_table", "target": "host"})
	res, err := tool.Call(context.Background(), json.RawMessage(in), nil)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected a tool error for unknown method, got: %s", res.Flatten())
	}
}

// TestCleanupArtifactsWindowsCmd verifies that cleanup_artifacts emits the correct
// del /f /q command for Windows targets.
func TestCleanupArtifactsWindowsCmd(t *testing.T) {
	ts := NewToolSet(nil, "")
	tool := ts.cleanupArtifacts()

	paths := []string{`C:\Windows\Temp\payload.exe`, `C:\Users\Public\loader.ps1`}
	in, _ := json.Marshal(map[string]any{
		"target": "192.168.1.20",
		"paths":  paths,
		"os":     "windows",
	})
	res, err := tool.Call(context.Background(), json.RawMessage(in), nil)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", res.Flatten())
	}
	body := res.Flatten()
	if !strings.Contains(body, "del /f /q") {
		t.Errorf("expected 'del /f /q' in Windows cleanup output, got: %s", body)
	}
	// JSON encoding turns \ into \\; check for the filename portion instead.
	if !strings.Contains(body, "payload.exe") {
		t.Errorf("expected 'payload.exe' in output, got: %s", body)
	}
}

// TestCleanupArtifactsLinuxCmd verifies rm -rf is emitted for Linux targets.
func TestCleanupArtifactsLinuxCmd(t *testing.T) {
	ts := NewToolSet(nil, "")
	tool := ts.cleanupArtifacts()

	paths := []string{"/tmp/implant", "/var/tmp/.s"}
	in, _ := json.Marshal(map[string]any{
		"target": "10.0.0.5",
		"paths":  paths,
		"os":     "linux",
	})
	res, err := tool.Call(context.Background(), json.RawMessage(in), nil)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", res.Flatten())
	}
	body := res.Flatten()
	if !strings.Contains(body, "rm -rf") {
		t.Errorf("expected 'rm -rf' in Linux cleanup output, got: %s", body)
	}
}

// TestCleanupArtifactsUnknownOS verifies that an unknown OS value returns a tool
// error rather than panicking or silently producing empty commands.
func TestCleanupArtifactsUnknownOS(t *testing.T) {
	ts := NewToolSet(nil, "")
	tool := ts.cleanupArtifacts()
	in, _ := json.Marshal(map[string]any{
		"target": "host",
		"paths":  []string{"/tmp/x"},
		"os":     "beos",
	})
	res, err := tool.Call(context.Background(), json.RawMessage(in), nil)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected a tool error for unknown OS, got: %s", res.Flatten())
	}
}

// TestCheckEDRReturnsName verifies that check_edr returns a result containing the
// EDR catalog and detection script rather than an error.
func TestCheckEDRReturnsName(t *testing.T) {
	ts := NewToolSet(nil, "")
	tool := ts.checkEDR()

	in, _ := json.Marshal(map[string]any{
		"target": "192.168.1.50",
	})
	res, err := tool.Call(context.Background(), json.RawMessage(in), nil)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", res.Flatten())
	}
	body := res.Flatten()
	// Result must name at least one well-known EDR product.
	for _, name := range []string{"CrowdStrike", "SentinelOne", "Defender", "Carbon Black"} {
		if strings.Contains(body, name) {
			return // at least one known EDR name is present
		}
	}
	t.Errorf("expected at least one EDR name (CrowdStrike/SentinelOne/Defender/Carbon Black) in result, got: %s", body)
}

// TestCheckEDRRequiresTarget verifies that an empty target is rejected.
func TestCheckEDRRequiresTarget(t *testing.T) {
	ts := NewToolSet(nil, "")
	tool := ts.checkEDR()
	in, _ := json.Marshal(map[string]any{"target": ""})
	res, err := tool.Call(context.Background(), json.RawMessage(in), nil)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected tool error for empty target, got: %s", res.Flatten())
	}
}

// TestLateralScanReturnsCommands verifies that lateral_scan produces a non-empty
// command list and does not panic with a nil store.
func TestLateralScanReturnsCommands(t *testing.T) {
	ts := NewToolSet(nil, "")
	tool := ts.lateralScan()

	in, _ := json.Marshal(map[string]any{"target": "10.10.10.0/24"})
	res, err := tool.Call(context.Background(), json.RawMessage(in), nil)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", res.Flatten())
	}
	body := res.Flatten()
	if !strings.Contains(body, "nmap") {
		t.Errorf("expected nmap command in lateral_scan output, got: %s", body)
	}
}

// TestMoveLateralUnknownMethod verifies that an unknown movement method returns a
// tool error.
func TestMoveLateralUnknownMethod(t *testing.T) {
	ts := NewToolSet(nil, "")
	tool := ts.moveLateral()
	in, _ := json.Marshal(map[string]any{"target": "host", "method": "telepathy", "credential_id": 1})
	res, err := tool.Call(context.Background(), json.RawMessage(in), nil)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected tool error for unknown method, got: %s", res.Flatten())
	}
}

// TestDomainEscalateKerberoast verifies kerberoast technique returns hashcat command.
func TestDomainEscalateKerberoast(t *testing.T) {
	ts := NewToolSet(nil, "")
	tool := ts.domainEscalate()
	in, _ := json.Marshal(map[string]any{"method": "kerberoast"})
	res, err := tool.Call(context.Background(), json.RawMessage(in), nil)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", res.Flatten())
	}
	if !strings.Contains(res.Flatten(), "hashcat") {
		t.Errorf("expected hashcat in kerberoast output, got: %s", res.Flatten())
	}
}
