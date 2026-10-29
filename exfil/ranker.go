package exfil

import "fmt"

// ExfilManifest is a ranked list of artifacts for operator review.
type ExfilManifest struct {
	TaskID    int64      `json:"task_id"`
	Total     int        `json:"total"`
	Artifacts []Artifact `json:"artifacts"`
}

// BuildManifest creates a ranked manifest from artifacts.
func BuildManifest(taskID int64, artifacts []Artifact) ExfilManifest {
	ranked := RankArtifacts(artifacts)
	return ExfilManifest{
		TaskID:    taskID,
		Total:     len(ranked),
		Artifacts: ranked,
	}
}

// Summary returns a human-readable summary line.
func (m ExfilManifest) Summary() string {
	types := make(map[string]int)
	for _, a := range m.Artifacts {
		types[a.Type]++
	}
	return fmt.Sprintf("Task %d: %d artifacts — %v", m.TaskID, m.Total, types)
}
