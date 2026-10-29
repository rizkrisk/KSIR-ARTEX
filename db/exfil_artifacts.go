package db

import (
	"time"
)

// ExfilArtifact represents an exfiltrated artifact record.
type ExfilArtifact struct {
	ID            int64     `json:"id"`
	TaskID        int64     `json:"task_id"`
	FindingID     *int64    `json:"finding_id,omitempty"`
	ArtifactType  string    `json:"artifact_type"`
	FilePath      string    `json:"file_path,omitempty"`
	FileSize      int64     `json:"file_size,omitempty"`
	HashSHA256    string    `json:"hash_sha256,omitempty"`
	CredentialRef *int64    `json:"credential_ref,omitempty"`
	Compressed    bool      `json:"compressed"`
	Encrypted     bool      `json:"encrypted"`
	StoredPath    string    `json:"stored_path,omitempty"`
	ValueRank     int       `json:"value_rank"`
	CreatedAt     time.Time `json:"created_at"`
}

// InsertExfilArtifact inserts a new exfil artifact and returns its ID.
func (d *DB) InsertExfilArtifact(a ExfilArtifact) (int64, error) {
	var id int64
	err := d.QueryRow(`
		INSERT INTO exfil_artifacts
		(task_id, finding_id, artifact_type, file_path, file_size, hash_sha256,
		 credential_ref, compressed, encrypted, stored_path, value_rank)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id`,
		a.TaskID, a.FindingID, a.ArtifactType, a.FilePath, a.FileSize, a.HashSHA256,
		a.CredentialRef, a.Compressed, a.Encrypted, a.StoredPath, a.ValueRank,
	).Scan(&id)
	return id, err
}

// ListExfilArtifacts returns all artifacts for a task, ordered by value_rank DESC.
func (d *DB) ListExfilArtifacts(taskID int64) ([]ExfilArtifact, error) {
	rows, err := d.Query(`
		SELECT id, task_id, finding_id, artifact_type, file_path, file_size, hash_sha256,
		       credential_ref, compressed, encrypted, stored_path, value_rank, created_at
		FROM exfil_artifacts WHERE task_id=$1 ORDER BY value_rank DESC, created_at DESC`,
		taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var artifacts []ExfilArtifact
	for rows.Next() {
		var a ExfilArtifact
		if err := rows.Scan(
			&a.ID, &a.TaskID, &a.FindingID, &a.ArtifactType, &a.FilePath,
			&a.FileSize, &a.HashSHA256, &a.CredentialRef, &a.Compressed, &a.Encrypted,
			&a.StoredPath, &a.ValueRank, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, a)
	}
	return artifacts, rows.Err()
}
