package event_schemas

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
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

type CompatibilityResult struct {
	Type            string   `json:"type"`
	Compatible      bool     `json:"compatible"`
	Reason          string   `json:"reason"`
	BreakingChanges []string `json:"breaking_changes"`
}

type schemaProperty struct {
	Type string `json:"type"`
}

type schemaDocument struct {
	Type                 string                    `json:"type"`
	Properties           map[string]schemaProperty `json:"properties"`
	Required             []string                  `json:"required"`
	AdditionalProperties *bool                     `json:"additionalProperties"`
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
	version, err := s.RegisterVersionStrict(eventType, schema, status)
	if err != nil {
		if schema == "" {
			schema = `{\"type\":\"object\"}`
		}
		version = Version{
			Type:    eventType,
			Version: len(s.ListVersions(eventType)) + 1,
			Schema:  schema,
			Status:  status,
		}
		if version.Status == "" {
			version.Status = "active"
		}
		s.mu.Lock()
		s.versions[eventType] = append(s.versions[eventType], version)
		s.mu.Unlock()
	}
	return version
}

func (s *Service) RegisterVersionStrict(eventType, schema, status string) (Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return Version{}, fmt.Errorf("event type is required")
	}
	if schema == "" {
		schema = `{"type":"object"}`
	}
	if status == "" {
		status = "active"
	}
	if _, err := parseSchema(schema); err != nil {
		return Version{}, fmt.Errorf("EVENT_SCHEMA_INVALID: %w", err)
	}

	compatibility := s.checkCompatibilityLocked(eventType, schema)
	if !compatibility.Compatible {
		return Version{}, fmt.Errorf("EVENT_SCHEMA_INCOMPATIBLE: %s", strings.Join(compatibility.BreakingChanges, "; "))
	}

	nextVersion := len(s.versions[eventType]) + 1
	version := Version{
		Type:    eventType,
		Version: nextVersion,
		Schema:  schema,
		Status:  status,
	}
	s.versions[eventType] = append(s.versions[eventType], version)
	return version, nil
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

func (s *Service) CheckCompatibility(eventType, schema string) CompatibilityResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.checkCompatibilityLocked(eventType, schema)
}

func (s *Service) checkCompatibilityLocked(eventType, schema string) CompatibilityResult {
	result := CompatibilityResult{
		Type:            eventType,
		Compatible:      true,
		Reason:          "COMPATIBLE",
		BreakingChanges: []string{},
	}
	versions := s.versions[eventType]
	if len(versions) == 0 {
		result.Reason = "INITIAL_VERSION"
		return result
	}

	newSchema, err := parseSchema(schema)
	if err != nil {
		result.Compatible = false
		result.Reason = "SCHEMA_INVALID"
		result.BreakingChanges = []string{err.Error()}
		return result
	}
	oldSchema, err := parseSchema(versions[len(versions)-1].Schema)
	if err != nil {
		result.Compatible = false
		result.Reason = "EXISTING_SCHEMA_INVALID"
		result.BreakingChanges = []string{err.Error()}
		return result
	}

	oldRequired := make(map[string]struct{}, len(oldSchema.Required))
	for _, key := range oldSchema.Required {
		oldRequired[key] = struct{}{}
	}
	newRequired := make(map[string]struct{}, len(newSchema.Required))
	for _, key := range newSchema.Required {
		newRequired[key] = struct{}{}
	}

	for oldField := range oldSchema.Properties {
		newProperty, exists := newSchema.Properties[oldField]
		if !exists {
			result.BreakingChanges = append(result.BreakingChanges, fmt.Sprintf("field removed: %s", oldField))
			continue
		}
		oldProperty := oldSchema.Properties[oldField]
		if oldProperty.Type != "" && newProperty.Type != "" && oldProperty.Type != newProperty.Type {
			result.BreakingChanges = append(result.BreakingChanges, fmt.Sprintf("field type changed: %s (%s -> %s)", oldField, oldProperty.Type, newProperty.Type))
		}
	}
	for requiredField := range oldRequired {
		if _, ok := newRequired[requiredField]; !ok {
			result.BreakingChanges = append(result.BreakingChanges, fmt.Sprintf("required field removed: %s", requiredField))
		}
	}

	if len(result.BreakingChanges) > 0 {
		result.Compatible = false
		result.Reason = "BREAKING_CHANGE"
	}
	return result
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

	schema, err := parseSchema(versions[requestedVersion-1].Schema)
	if err != nil {
		return ValidationResult{
			Type:    eventType,
			Version: requestedVersion,
			Valid:   false,
			Reason:  "EVENT_SCHEMA_INVALID",
		}
	}
	if reason := validateEventAgainstSchema(schema, event); reason != "" {
		return ValidationResult{
			Type:    eventType,
			Version: requestedVersion,
			Valid:   false,
			Reason:  reason,
		}
	}

	return ValidationResult{
		Type:    eventType,
		Version: requestedVersion,
		Valid:   true,
		Reason:  "ok",
	}
}

func parseSchema(raw string) (schemaDocument, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = `{"type":"object"}`
	}

	var parsed schemaDocument
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return schemaDocument{}, fmt.Errorf("invalid json schema payload")
	}
	if parsed.Type == "" {
		parsed.Type = "object"
	}
	if parsed.Type != "object" {
		return schemaDocument{}, fmt.Errorf("schema root type must be object")
	}
	if parsed.Properties == nil {
		parsed.Properties = map[string]schemaProperty{}
	}
	if parsed.Required == nil {
		parsed.Required = []string{}
	}
	return parsed, nil
}

func validateEventAgainstSchema(schema schemaDocument, event map[string]any) string {
	required := make(map[string]struct{}, len(schema.Required))
	for _, field := range schema.Required {
		required[field] = struct{}{}
	}
	for field := range required {
		if _, ok := event[field]; !ok {
			return "EVENT_SCHEMA_REQUIRED_FIELD_MISSING_" + field
		}
	}

	if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
		for field := range event {
			if _, ok := schema.Properties[field]; !ok {
				return "EVENT_SCHEMA_UNKNOWN_FIELD_" + field
			}
		}
	}

	for field, value := range event {
		property, ok := schema.Properties[field]
		if !ok {
			continue
		}
		if property.Type == "" {
			continue
		}
		if !matchesJSONType(value, property.Type) {
			return "EVENT_SCHEMA_TYPE_MISMATCH_" + field
		}
	}
	return ""
}

func matchesJSONType(value any, expected string) bool {
	switch expected {
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "number":
		switch value.(type) {
		case float64, float32, int, int64, int32, uint, uint64, uint32:
			return true
		default:
			return false
		}
	case "integer":
		switch typed := value.(type) {
		case int, int64, int32, uint, uint64, uint32:
			return true
		case float64:
			return math.Trunc(typed) == typed
		case float32:
			return math.Trunc(float64(typed)) == float64(typed)
		default:
			return false
		}
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	default:
		return true
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
