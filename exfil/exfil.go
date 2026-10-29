package exfil

import "sort"

// Artifact represents an exfiltrated artifact.
type Artifact struct {
	ID         int64
	TaskID     int64
	Type       string  // credential/dump/screenshot/source/config/file/token
	Path       string
	SizeMB     float64
	SHA256     string
	ValueRank  int
	StoredPath string
}

// artifactScore maps artifact types to their value score.
var artifactScore = map[string]int{
	"credential": 100,
	"token":      90,
	"config":     80,
	"dump":       60,
	"source":     40,
	"screenshot": 20,
	"file":       10,
}

// RankArtifacts sorts artifacts by value rank descending.
// Credential > Token > Config > Dump > Source > Screenshot > File
func RankArtifacts(artifacts []Artifact) []Artifact {
	for i := range artifacts {
		if score, ok := artifactScore[artifacts[i].Type]; ok {
			artifacts[i].ValueRank = score
		}
	}
	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].ValueRank > artifacts[j].ValueRank
	})
	return artifacts
}
