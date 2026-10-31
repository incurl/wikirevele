package jobs

import (
	"fmt"
	"io"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Artifact is the frontend-facing view of a stored file.
//
// The queue models no specific kind of file: an artifact is whatever
// the application stored, with the metadata the application recorded.
type Artifact struct {
	// ID is the PocketBase record id.
	ID string `json:"id"`
	// Kind is "input" or "output".
	Kind string `json:"kind"`
	// JobID is the owning job, if any.
	JobID string `json:"job_id"`
	// URL is the native PocketBase file URL.
	URL string `json:"url"`
	// Name is the file name.
	Name string `json:"name"`
	// Mime is the stored MIME type.
	Mime string `json:"mime"`
	// Bytes is the file size.
	Bytes int `json:"bytes"`
	// Meta is the application-specific metadata.
	Meta map[string]any `json:"meta"`
	// Created is the ISO-8601 creation timestamp.
	Created string `json:"created"`
}

// ListArtifacts returns the stored artifacts, newest first.
//
// When jobID is given, only the artifacts of that job are returned.
func (m *Manager) ListArtifacts(jobID string, limit int) ([]Artifact, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	filter := ""
	var params []dbx.Params
	if jobID != "" {
		filter = "job = {:job}"
		params = append(params, dbx.Params{"job": jobID})
	}

	records, err := m.app.FindRecordsByFilter(
		CollectionArtifacts,
		filter,
		"-created",
		limit,
		0,
		params...,
	)
	if err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}

	artifacts := make([]Artifact, 0, len(records))
	for _, record := range records {
		artifacts = append(artifacts, m.toArtifact(record))
	}
	return artifacts, nil
}

// GetArtifact returns a single artifact.
func (m *Manager) GetArtifact(artifactID string) (Artifact, error) {
	record, err := m.app.FindRecordById(CollectionArtifacts, artifactID)
	if err != nil {
		return Artifact{}, fmt.Errorf("find artifact %q: %w", artifactID, err)
	}
	return m.toArtifact(record), nil
}

// DeleteArtifact removes a stored artifact.
func (m *Manager) DeleteArtifact(artifactID string) error {
	record, err := m.app.FindRecordById(CollectionArtifacts, artifactID)
	if err != nil {
		return fmt.Errorf("find artifact %q: %w", artifactID, err)
	}
	if err := m.app.Delete(record); err != nil {
		return fmt.Errorf("delete artifact %q: %w", artifactID, err)
	}
	return nil
}

// ArtifactBytes reads the bytes of a stored artifact.
//
// It exists for the Runner: a runner that the queue handed an
// InputArtifactIDs list needs to turn an id into the file. Nothing else
// should reach for it -- the frontend reads a file by its URL, which
// PocketBase serves.
func (m *Manager) ArtifactBytes(artifactID string) ([]byte, error) {
	record, err := m.app.FindRecordById(CollectionArtifacts, artifactID)
	if err != nil {
		return nil, fmt.Errorf("find artifact %q: %w", artifactID, err)
	}
	return readFileBytes(m.app, record)
}

// toArtifact converts a PocketBase record into the frontend-facing
// artifact.
func (m *Manager) toArtifact(record *core.Record) Artifact {
	artifact := Artifact{
		ID:      record.Id,
		Kind:    record.GetString("kind"),
		JobID:   record.GetString("job"),
		Name:    record.GetString("name"),
		Mime:    record.GetString("mime"),
		Bytes:   record.GetInt("bytes"),
		Meta:    map[string]any{},
		Created: record.GetString("created"),
	}

	if filename := record.GetString("file"); filename != "" {
		// PocketBase stores the served file name in the file field. The
		// URL is built from it, so that the link always points at the
		// stored file and never at the original upload name.
		artifact.URL = fmt.Sprintf("/api/files/%s/%s/%s", CollectionArtifacts, record.Id, filename)
	}

	return artifact
}

// readFileBytes reads the bytes of a record's file attachment.
//
// PocketBase stores file fields as the generated filename on the record,
// and keeps the bytes in the collection's storage directory. The
// filesystem is therefore resolved through the app, exactly as the
// built-in file route does.
func readFileBytes(app core.App, record *core.Record) ([]byte, error) {
	filename := record.GetString("file")
	if filename == "" {
		return nil, fmt.Errorf("record has no file")
	}

	fsys, err := app.NewFilesystem()
	if err != nil {
		return nil, fmt.Errorf("create filesystem: %w", err)
	}
	defer func() { _ = fsys.Close() }()

	reader, err := fsys.GetReader(record.BaseFilesPath() + "/" + filename)
	if err != nil {
		return nil, fmt.Errorf("open file %q: %w", filename, err)
	}
	defer func() { _ = reader.Close() }()

	return io.ReadAll(reader)
}
