package knowledge

import (
	"fmt"
	"sync"
	"time"
)

// KnowledgeFile represents a tracked knowledge file.
type KnowledgeFile struct {
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspace_id"`
	FileType      string    `json:"file_type"` // USER.md, SOUL.md, AGENTS.md
	ContentHash   string    `json:"content_hash"`
	LastCheckedAt time.Time `json:"last_checked_at"`
	DriftDetected bool      `json:"drift_detected"`
}

// DriftResult describes the outcome of a drift check.
type DriftResult struct {
	Drifted       bool      `json:"drifted"`
	PreviousHash  string    `json:"previous_hash"`
	CurrentHash   string    `json:"current_hash"`
	LastCheckedAt time.Time `json:"last_checked_at"`
}

// DriftDetector detects content drift in knowledge files.
type DriftDetector struct {
	mu    sync.RWMutex
	files map[string]*KnowledgeFile // key: "workspaceID:fileType"
	now   func() time.Time
}

// NewDriftDetector creates a new drift detector.
func NewDriftDetector() *DriftDetector {
	return &DriftDetector{
		files: map[string]*KnowledgeFile{},
		now:   func() time.Time { return time.Now().UTC() },
	}
}

func driftKey(workspaceID, fileType string) string {
	return workspaceID + ":" + fileType
}

// RegisterFile registers or updates a knowledge file for drift detection.
func (d *DriftDetector) RegisterFile(workspaceID, fileType, contentHash string) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if fileType == "" {
		return fmt.Errorf("file_type is required")
	}
	if contentHash == "" {
		return fmt.Errorf("content_hash is required")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	key := driftKey(workspaceID, fileType)
	d.files[key] = &KnowledgeFile{
		ID:            key,
		WorkspaceID:   workspaceID,
		FileType:      fileType,
		ContentHash:   contentHash,
		LastCheckedAt: d.now(),
		DriftDetected: false,
	}
	return nil
}

// CheckDrift compares the current hash against the stored hash.
func (d *DriftDetector) CheckDrift(workspaceID, fileType, currentHash string) (*DriftResult, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if fileType == "" {
		return nil, fmt.Errorf("file_type is required")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	key := driftKey(workspaceID, fileType)
	file, ok := d.files[key]
	if !ok {
		return nil, fmt.Errorf("file %q not registered for workspace %q", fileType, workspaceID)
	}

	now := d.now()
	drifted := file.ContentHash != currentHash

	result := &DriftResult{
		Drifted:       drifted,
		PreviousHash:  file.ContentHash,
		CurrentHash:   currentHash,
		LastCheckedAt: now,
	}

	// Update tracked file
	file.DriftDetected = drifted
	file.LastCheckedAt = now
	if drifted {
		file.ContentHash = currentHash
	}

	return result, nil
}

// GetFile returns the tracked knowledge file, if any.
func (d *DriftDetector) GetFile(workspaceID, fileType string) (*KnowledgeFile, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	f, ok := d.files[driftKey(workspaceID, fileType)]
	if !ok {
		return nil, false
	}
	cp := *f
	return &cp, true
}
