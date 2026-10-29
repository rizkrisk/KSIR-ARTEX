package c2bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageImplantFoundX64(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "implant_x64.exe")
	if err := os.WriteFile(binPath, []byte("MZ stub payload x64"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		BinaryX64:  binPath,
		BinaryX86:  filepath.Join(dir, "implant_x86.exe"),
		StagingDir: dir,
	}
	b := NewLocalBackend(cfg)
	path, err := b.Stage(context.Background(), "x64")
	if err != nil {
		t.Fatalf("Stage returned error: %v", err)
	}
	if path != binPath {
		t.Fatalf("expected path %q, got %q", binPath, path)
	}
}

func TestStageImplantMissingX86(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		BinaryX64:  filepath.Join(dir, "implant_x64.exe"),
		BinaryX86:  filepath.Join(dir, "implant_x86.exe"),
		StagingDir: dir,
	}
	b := NewLocalBackend(cfg)
	_, err := b.Stage(context.Background(), "x86")
	if err == nil {
		t.Fatal("expected error for missing x86 binary, got nil")
	}
}

func TestDeliverViaEchoB64(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "implant_x64.exe")
	if err := os.WriteFile(binPath, []byte("MZ stub payload for echo_b64 test"), 0644); err != nil {
		t.Fatal(err)
	}
	vector := DeliveryVector{
		Type:       "echo_b64",
		TargetPath: `C:\Windows\Temp\svc.exe`,
	}
	cmd := BuildDeliveryCommand(vector, binPath)
	if !strings.Contains(cmd, "certutil") {
		t.Fatalf("echo_b64 command missing certutil decode chain:\n%s", cmd)
	}
	if !strings.Contains(cmd, "echo") {
		t.Fatalf("echo_b64 command missing echo statement:\n%s", cmd)
	}
	if strings.Contains(cmd, "binary not found") {
		t.Fatalf("echo_b64 returned error path instead of encoding:\n%s", cmd)
	}
}

func TestSelectDeliveryVector(t *testing.T) {
	tests := []struct {
		shellType string
		wantType  string
	}{
		{"webshell_php", "webshell_upload"},
		{"rce_cmd", "echo_b64"},
		{"sqli_exec", "echo_b64"},
		{"ssh", "wget"},
	}
	for _, tc := range tests {
		v := SelectDeliveryVector(tc.shellType)
		if v.Type != tc.wantType {
			t.Errorf("SelectDeliveryVector(%q): got Type=%q, want %q", tc.shellType, v.Type, tc.wantType)
		}
	}
}
