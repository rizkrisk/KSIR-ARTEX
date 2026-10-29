package exfil

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CollectFiles scans a task work directory for exfiltrated files matching pattern.
func CollectFiles(taskID int64, workDir, pattern string) ([]Artifact, error) {
	taskDir := filepath.Join(workDir, "tasks", fmt.Sprintf("%d", taskID))
	var artifacts []Artifact

	err := filepath.Walk(taskDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if pattern != "" {
			matched, _ := filepath.Match(pattern, filepath.Base(path))
			if !matched {
				return nil
			}
		}
		hash := ""
		if f, err := os.Open(path); err == nil {
			h := sha256.New()
			io.Copy(h, f) //nolint:errcheck
			f.Close()
			hash = fmt.Sprintf("%x", h.Sum(nil))
		}
		artifacts = append(artifacts, Artifact{
			TaskID:     taskID,
			Type:       classifyFile(path),
			Path:       path,
			SizeMB:     float64(info.Size()) / (1024 * 1024),
			SHA256:     hash,
			StoredPath: path,
		})
		return nil
	})
	return RankArtifacts(artifacts), err
}

// classifyFile guesses artifact type by file extension/name.
func classifyFile(path string) string {
	base := filepath.Base(path)
	switch {
	case matchAny(base, "*.cred", "*.credentials", "*.passwd", "shadow", "SAM", "NTDS.dit"):
		return "credential"
	case matchAny(base, "*.config", "*.cfg", "*.env", "web.config", "appsettings.json"):
		return "config"
	case matchAny(base, "*.dmp", "*.dump", "lsass*"):
		return "dump"
	case matchAny(base, "*.png", "*.jpg", "*.jpeg", "screenshot*"):
		return "screenshot"
	case matchAny(base, "*.go", "*.py", "*.js", "*.php", "*.java", "*.cs"):
		return "source"
	case matchAny(base, "*.jwt", "*.token", "*.key"):
		return "token"
	default:
		return "file"
	}
}

func matchAny(name string, patterns ...string) bool {
	for _, p := range patterns {
		if matched, _ := filepath.Match(p, name); matched {
			return true
		}
	}
	return false
}
