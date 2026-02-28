package structured_generation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	toolKeyPattern       = regexp.MustCompile(`^[a-z0-9_]+\.[a-z0-9_]+$`)
	idempotencyKeyFormat = regexp.MustCompile(`^idem_[0-9a-v]{16,}$`)
)

type Action struct {
	Tool           string         `json:"tool"`
	Operation      string         `json:"operation"`
	Params         map[string]any `json:"params"`
	IdempotencyKey string         `json:"idempotency_key"`
}

type Risk struct {
	Impact       string `json:"impact"`
	RollbackPlan string `json:"rollback_plan"`
}

type ActionProposal struct {
	Intent           string   `json:"intent"`
	Actions          []Action `json:"actions"`
	Risk             Risk     `json:"risk"`
	RequiresApproval bool     `json:"requires_approval"`
}

type ConstraintConfig struct {
	MaxActions int
}

type Service struct {
	config ConstraintConfig
}

func NewService() *Service {
	return &Service{
		config: ConstraintConfig{
			MaxActions: 8,
		},
	}
}

func (s *Service) CanonicalizeProposal(proposal ActionProposal) (ActionProposal, error) {
	if err := s.ValidateProposal(proposal); err != nil {
		return ActionProposal{}, err
	}

	out := ActionProposal{
		Intent:           strings.TrimSpace(proposal.Intent),
		Actions:          make([]Action, 0, len(proposal.Actions)),
		Risk:             proposal.Risk,
		RequiresApproval: proposal.RequiresApproval,
	}
	for _, action := range proposal.Actions {
		out.Actions = append(out.Actions, Action{
			Tool:           strings.TrimSpace(action.Tool),
			Operation:      strings.TrimSpace(action.Operation),
			Params:         cloneParams(action.Params),
			IdempotencyKey: strings.TrimSpace(action.IdempotencyKey),
		})
	}
	sort.Slice(out.Actions, func(i, j int) bool {
		left := out.Actions[i].Tool + "::" + out.Actions[i].Operation + "::" + out.Actions[i].IdempotencyKey
		right := out.Actions[j].Tool + "::" + out.Actions[j].Operation + "::" + out.Actions[j].IdempotencyKey
		return left < right
	})
	return out, nil
}

func (s *Service) CanonicalJSON(proposal ActionProposal) (string, error) {
	canonical, err := s.CanonicalizeProposal(proposal)
	if err != nil {
		return "", err
	}
	blob, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	return string(blob), nil
}

func (s *Service) ValidateProposal(proposal ActionProposal) error {
	intent := strings.TrimSpace(proposal.Intent)
	if intent == "" {
		return fmt.Errorf("intent is required")
	}
	if len(proposal.Actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}
	if len(proposal.Actions) > s.config.MaxActions {
		return fmt.Errorf("too many actions: %d > %d", len(proposal.Actions), s.config.MaxActions)
	}
	if strings.TrimSpace(proposal.Risk.Impact) == "" || strings.TrimSpace(proposal.Risk.RollbackPlan) == "" {
		return fmt.Errorf("risk impact and rollback_plan are required")
	}
	for _, action := range proposal.Actions {
		if !toolKeyPattern.MatchString(strings.TrimSpace(action.Tool)) {
			return fmt.Errorf("invalid action tool key: %s", action.Tool)
		}
		if strings.TrimSpace(action.Operation) == "" {
			return fmt.Errorf("action operation is required")
		}
		if !idempotencyKeyFormat.MatchString(strings.TrimSpace(action.IdempotencyKey)) {
			return fmt.Errorf("invalid idempotency key: %s", action.IdempotencyKey)
		}
	}
	return nil
}

func cloneParams(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
