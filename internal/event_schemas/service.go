package event_schemas

import (
	"fmt"
	"sort"
	"sync"
)

type EventType struct {
	Type          string `json:"type"`
	LatestVersion int    `json:"latest_version"`
}

type Version struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
	Schema  string `json:"schema"`
	Status  string `json:"status"`
}

type ValidationResult struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
	Valid   bool   `json:"valid"`
	Reason  string `json:"reason"`
}

type Service struct {
	mu       sync.RWMutex
	versions map[string][]Version
}

func NewService() *Service {
	return &Service{
		versions: map[string][]Version{},
	}
}

func (s *Service) RegisterVersion(eventType, schema, status string) Version {
	s.mu.Lock()
	defer s.mu.Unlock()

	if schema == "" {
		schema = "{}"
	}
	if status == "" {
		status = "active"
	}
	nextVersion := len(s.versions[eventType]) + 1
	version := Version{
		Type:    eventType,
		Version: nextVersion,
		Schema:  schema,
		Status:  status,
	}
	s.versions[eventType] = append(s.versions[eventType], version)
	return version
}

func (s *Service) ListTypes() []EventType {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]EventType, 0, len(s.versions))
	for eventType, versions := range s.versions {
		out = append(out, EventType{
			Type:          eventType,
			LatestVersion: len(versions),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Type < out[j].Type
	})
	return out
}

func (s *Service) ListVersions(eventType string) []Version {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Version, len(s.versions[eventType]))
	copy(out, s.versions[eventType])
	return out
}

func (s *Service) Validate(eventType string, event map[string]any) ValidationResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions := s.versions[eventType]
	if len(versions) == 0 {
		return ValidationResult{
			Type:   eventType,
			Valid:  false,
			Reason: "EVENT_TYPE_NOT_REGISTERED",
		}
	}

	typeField, _ := event["type"].(string)
	if typeField != eventType {
		return ValidationResult{
			Type:   eventType,
			Valid:  false,
			Reason: "EVENT_TYPE_MISMATCH",
		}
	}

	rawVersion, hasVersion := event["version"]
	if !hasVersion {
		return ValidationResult{
			Type:   eventType,
			Valid:  false,
			Reason: "EVENT_VERSION_MISSING",
		}
	}

	requestedVersion, ok := coerceVersion(rawVersion)
	if !ok {
		return ValidationResult{
			Type:   eventType,
			Valid:  false,
			Reason: "EVENT_VERSION_INVALID",
		}
	}
	if requestedVersion < 1 || requestedVersion > len(versions) {
		return ValidationResult{
			Type:    eventType,
			Version: requestedVersion,
			Valid:   false,
			Reason:  "EVENT_VERSION_OUT_OF_RANGE",
		}
	}

	return ValidationResult{
		Type:    eventType,
		Version: requestedVersion,
		Valid:   true,
		Reason:  "ok",
	}
}

func coerceVersion(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		var parsed int
		if _, err := fmt.Sscanf(typed, "%d", &parsed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}
