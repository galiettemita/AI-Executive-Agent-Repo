package temporal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// InitVoiceSessionActivity initializes a voice session with the provider.
func InitVoiceSessionActivity(_ context.Context, input VoiceInitInput) (*VoiceInitResult, error) {
	if input.SessionID == "" || input.WorkspaceID == "" {
		return nil, fmt.Errorf("session_id and workspace_id required")
	}

	// Generate a deterministic room name
	h := sha256.Sum256([]byte(input.SessionID + ":" + input.WorkspaceID))
	roomName := "voice-" + hex.EncodeToString(h[:8])

	return &VoiceInitResult{
		Token:    fmt.Sprintf("tok_%s_%s", input.SessionID[:8], input.ChannelType),
		RoomName: roomName,
	}, nil
}

// ExtractVoiceTasksActivity extracts actionable tasks from a voice transcript.
func ExtractVoiceTasksActivity(_ context.Context, input VoiceTaskExtractInput) (*VoiceTaskExtractResult, error) {
	if input.Transcript == "" {
		return &VoiceTaskExtractResult{Tasks: []string{}}, nil
	}

	// Pattern-based task extraction from transcript
	patterns := []string{"remind me to", "schedule", "send", "follow up", "set up", "create", "book"}
	sentences := strings.Split(input.Transcript, ".")
	var tasks []string

	for _, sentence := range sentences {
		lower := strings.ToLower(strings.TrimSpace(sentence))
		for _, pattern := range patterns {
			if strings.Contains(lower, pattern) {
				tasks = append(tasks, strings.TrimSpace(sentence))
				break
			}
		}
	}

	return &VoiceTaskExtractResult{Tasks: tasks}, nil
}
