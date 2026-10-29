package c2bridge

import "os"

// LoadConfig reads from environment variables or falls back to defaults.
// Env vars: ARTEX_C2_BIN_X64, ARTEX_C2_BIN_X86, ARTEX_C2_STAGING_DIR
func LoadConfig() Config {
	cfg := Config{
		BinaryX64:  os.Getenv("ARTEX_C2_BIN_X64"),
		BinaryX86:  os.Getenv("ARTEX_C2_BIN_X86"),
		StagingDir: os.Getenv("ARTEX_C2_STAGING_DIR"),
	}
	if cfg.BinaryX64 == "" {
		cfg.BinaryX64 = "data/c2/implant_x64.exe"
	}
	if cfg.BinaryX86 == "" {
		cfg.BinaryX86 = "data/c2/implant_x86.exe"
	}
	if cfg.StagingDir == "" {
		cfg.StagingDir = "data/c2"
	}
	return cfg
}
