package onboarding

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type Question struct {
	Key    string
	Prompt string
}

type StageResult struct {
	StageKey  string
	Extracted map[string]string
}

type WorkspaceProfile struct {
	WorkspaceID string
	VersionInt  int
	Dimensions  map[string]string
}

type WorkspacePersona struct {
	WorkspaceID string
	VersionInt  int
	Persona     map[string]string
}

type WorkspaceBehaviorPolicy struct {
	WorkspaceID string
	VersionInt  int
	Policy      map[string]string
}

type FollowupRule struct {
	RuleID      string
	WorkspaceID string
	Trigger     string
	Question    string
	Status      string
}

type AdaptiveQuestion struct {
	FollowupID  string
	WorkspaceID string
	Question    string
	Trigger     string
	Status      string
	Answer      string
}

type OnboardingButton struct {
	ButtonID string
	Label    string
	Action   string
}

type ConnectionTemplate struct {
	TemplateKey string
	Channel     string
	Title       string
	Body        string
	Buttons     []OnboardingButton
}

type Service struct {
	mu                sync.Mutex
	questionSets      map[string][]Question
	replay            map[string]map[string]string
	profiles          map[string]WorkspaceProfile
	personas          map[string]WorkspacePersona
	behaviorPolicies  map[string]WorkspaceBehaviorPolicy
	followupRules     map[string]map[string]FollowupRule
	adaptiveQuestions map[string]map[string]AdaptiveQuestion
	connectionCards   map[string]map[string]ConnectionTemplate
	nextFollowupID    int
}

func NewService() *Service {
	return &Service{
		questionSets: map[string][]Question{
			"operator_profile_intake_v1": {
				{Key: "role", Prompt: "What is your role?"},
				{Key: "goals", Prompt: "What are your primary goals?"},
				{Key: "industry", Prompt: "What industry does your workspace serve?"},
				{Key: "team_size", Prompt: "What is your team size?"},
				{Key: "timezone", Prompt: "What is your default timezone?"},
				{Key: "decision_style", Prompt: "How do you make decisions?"},
				{Key: "communication_pref", Prompt: "Preferred communication style?"},
				{Key: "kpi_primary", Prompt: "What is the primary KPI?"},
			},
			"behavior_policy_calibration_v1": {
				{Key: "tone", Prompt: "Preferred assistant tone?"},
				{Key: "risk_tolerance", Prompt: "Risk tolerance?"},
				{Key: "autonomy_preference", Prompt: "Autonomy preference?"},
				{Key: "approval_threshold", Prompt: "When should approvals be required?"},
				{Key: "proactive_mode", Prompt: "Should proactive actions be enabled?"},
				{Key: "notification_window", Prompt: "Preferred notification window?"},
				{Key: "initiative_level", Prompt: "How proactive should assistant initiative be?"},
			},
			"codebase_map_ingestion_v1": {
				{Key: "repo", Prompt: "Primary repository?"},
				{Key: "stack", Prompt: "Core stack?"},
				{Key: "planning_horizon", Prompt: "Planning horizon?"},
				{Key: "meeting_load", Prompt: "Weekly meeting load?"},
				{Key: "focus_mode", Prompt: "Preferred focus mode?"},
			},
			"system_map_ingestion_v1": {
				{Key: "integrations", Prompt: "Critical integrations?"},
				{Key: "sla", Prompt: "Critical SLA targets?"},
				{Key: "escalation_path", Prompt: "Escalation path?"},
				{Key: "privacy_mode", Prompt: "Privacy mode?"},
				{Key: "audit_strictness", Prompt: "Audit strictness?"},
				{Key: "delivery_cadence", Prompt: "Delivery cadence?"},
				{Key: "context_budget", Prompt: "Context budget preference?"},
				{Key: "write_actions", Prompt: "Write action policy?"},
				{Key: "language", Prompt: "Preferred language?"},
			},
		},
		replay:            map[string]map[string]string{},
		profiles:          map[string]WorkspaceProfile{},
		personas:          map[string]WorkspacePersona{},
		behaviorPolicies:  map[string]WorkspaceBehaviorPolicy{},
		followupRules:     map[string]map[string]FollowupRule{},
		adaptiveQuestions: map[string]map[string]AdaptiveQuestion{},
		connectionCards: map[string]map[string]ConnectionTemplate{
			"whatsapp": {
				"ecosystem_detect_v1": {
					TemplateKey: "ecosystem_detect_v1",
					Channel:     "whatsapp",
					Title:       "Connect {{app_name}}",
					Body:        "I noticed {{ecosystem_hint}}. Connect {{app_name}} to unlock automated workflows.",
					Buttons: []OnboardingButton{
						{ButtonID: "connect_now", Label: "Connect {{app_name}}", Action: "connect_app"},
						{ButtonID: "learn_more", Label: "How it works", Action: "view_connection_guide"},
						{ButtonID: "skip_now", Label: "Not now", Action: "skip_connection"},
					},
				},
				"connection_success_v1": {
					TemplateKey: "connection_success_v1",
					Channel:     "whatsapp",
					Title:       "{{app_name}} connected",
					Body:        "{{app_name}} is ready. Would you like a quick setup checklist?",
					Buttons: []OnboardingButton{
						{ButtonID: "start_checklist", Label: "Start setup", Action: "start_setup_checklist"},
						{ButtonID: "done", Label: "Done", Action: "dismiss"},
					},
				},
			},
			"imessage": {
				"ecosystem_detect_v1": {
					TemplateKey: "ecosystem_detect_v1",
					Channel:     "imessage",
					Title:       "Connect {{app_name}}",
					Body:        "Detected {{ecosystem_hint}}. Link {{app_name}} so I can prep actions for your approval.",
					Buttons: []OnboardingButton{
						{ButtonID: "connect_now", Label: "Connect {{app_name}}", Action: "connect_app"},
						{ButtonID: "later", Label: "Later", Action: "skip_connection"},
					},
				},
			},
		},
		nextFollowupID: 1,
	}
}

func replayKey(workspaceID, stageKey string, answers map[string]string) string {
	keys := make([]string, 0, len(answers))
	for key := range answers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{workspaceID, stageKey}
	for _, key := range keys {
		parts = append(parts, key+"="+answers[key])
	}
	return strings.Join(parts, "::")
}

func (s *Service) QuestionSet(stageKey string) ([]Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	questions, ok := s.questionSets[stageKey]
	if !ok {
		return nil, fmt.Errorf("unknown stage: %s", stageKey)
	}
	out := make([]Question, len(questions))
	copy(out, questions)
	return out, nil
}

func (s *Service) RunStage(workspaceID, stageKey string, answers map[string]string) (StageResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	questions, ok := s.questionSets[stageKey]
	if !ok {
		return StageResult{}, fmt.Errorf("unknown stage: %s", stageKey)
	}
	for _, q := range questions {
		if strings.TrimSpace(answers[q.Key]) == "" {
			return StageResult{}, fmt.Errorf("missing answer for %s", q.Key)
		}
	}

	key := replayKey(workspaceID, stageKey, answers)
	if cached, ok := s.replay[key]; ok {
		return StageResult{StageKey: stageKey, Extracted: copyStringMap(cached)}, nil
	}

	extracted := map[string]string{}
	for _, q := range questions {
		extracted[q.Key] = strings.TrimSpace(answers[q.Key])
	}
	s.replay[key] = copyStringMap(extracted)
	return StageResult{StageKey: stageKey, Extracted: extracted}, nil
}

func copyStringMap(input map[string]string) map[string]string {
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func nextVersion(current int) int {
	if current < 1 {
		return 1
	}
	return current + 1
}

func (s *Service) CompleteOnboarding(workspaceID string, stageAnswers map[string]map[string]string) error {
	stages := []string{
		"operator_profile_intake_v1",
		"behavior_policy_calibration_v1",
		"codebase_map_ingestion_v1",
		"system_map_ingestion_v1",
	}
	results := map[string]StageResult{}
	for _, stage := range stages {
		result, err := s.RunStage(workspaceID, stage, stageAnswers[stage])
		if err != nil {
			return err
		}
		results[stage] = result
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	profileVersion := nextVersion(s.profiles[workspaceID].VersionInt)
	personaVersion := nextVersion(s.personas[workspaceID].VersionInt)
	policyVersion := nextVersion(s.behaviorPolicies[workspaceID].VersionInt)

	op := results["operator_profile_intake_v1"].Extracted
	bp := results["behavior_policy_calibration_v1"].Extracted
	cb := results["codebase_map_ingestion_v1"].Extracted
	sy := results["system_map_ingestion_v1"].Extracted

	s.profiles[workspaceID] = WorkspaceProfile{
		WorkspaceID: workspaceID,
		VersionInt:  profileVersion,
		Dimensions: map[string]string{
			"role":                op["role"],
			"goals":               op["goals"],
			"industry":            op["industry"],
			"team_size":           op["team_size"],
			"timezone":            op["timezone"],
			"decision_style":      op["decision_style"],
			"communication_pref":  op["communication_pref"],
			"kpi_primary":         op["kpi_primary"],
			"risk_tolerance":      bp["risk_tolerance"],
			"autonomy_preference": bp["autonomy_preference"],
			"planning_horizon":    cb["planning_horizon"],
			"meeting_load":        cb["meeting_load"],
			"focus_mode":          cb["focus_mode"],
		},
	}
	s.personas[workspaceID] = WorkspacePersona{
		WorkspaceID: workspaceID,
		VersionInt:  personaVersion,
		Persona: map[string]string{
			"tone":               bp["tone"],
			"initiative_level":   bp["initiative_level"],
			"language":           sy["language"],
			"communication_pref": op["communication_pref"],
			"decision_style":     op["decision_style"],
		},
	}
	s.behaviorPolicies[workspaceID] = WorkspaceBehaviorPolicy{
		WorkspaceID: workspaceID,
		VersionInt:  policyVersion,
		Policy: map[string]string{
			"approval_threshold":  bp["approval_threshold"],
			"proactive_mode":      bp["proactive_mode"],
			"notification_window": bp["notification_window"],
			"write_actions":       sy["write_actions"],
			"escalation_path":     sy["escalation_path"],
			"privacy_mode":        sy["privacy_mode"],
			"audit_strictness":    sy["audit_strictness"],
			"delivery_cadence":    sy["delivery_cadence"],
			"context_budget":      sy["context_budget"],
			"sla":                 sy["sla"],
		},
	}
	s.ensureWorkspaceFollowupRulesLocked(workspaceID)
	s.generateAdaptiveQuestionsLocked(workspaceID, op, bp, cb, sy)
	return nil
}

func (s *Service) WorkspaceState(workspaceID string) (WorkspaceProfile, WorkspacePersona, WorkspaceBehaviorPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profile, okProfile := s.profiles[workspaceID]
	persona, okPersona := s.personas[workspaceID]
	policy, okPolicy := s.behaviorPolicies[workspaceID]
	if !okProfile || !okPersona || !okPolicy {
		return WorkspaceProfile{}, WorkspacePersona{}, WorkspaceBehaviorPolicy{}, fmt.Errorf("workspace onboarding state incomplete")
	}
	return profile, persona, policy, nil
}

func NewWorkspaceID() string {
	return uuid.Must(uuid.NewV7()).String()
}

func (s *Service) ListConnectionTemplates(channel string) []ConnectionTemplate {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedChannel := strings.ToLower(strings.TrimSpace(channel))
	templatesByKey, ok := s.connectionCards[normalizedChannel]
	if !ok {
		return nil
	}
	out := make([]ConnectionTemplate, 0, len(templatesByKey))
	for _, template := range templatesByKey {
		out = append(out, copyConnectionTemplate(template))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].TemplateKey < out[j].TemplateKey
	})
	return out
}

func (s *Service) RenderConnectionTemplate(channel, templateKey string, params map[string]string) (ConnectionTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedChannel := strings.ToLower(strings.TrimSpace(channel))
	normalizedTemplateKey := strings.TrimSpace(templateKey)
	templatesByKey, ok := s.connectionCards[normalizedChannel]
	if !ok {
		return ConnectionTemplate{}, fmt.Errorf("connection template channel not found: %s", normalizedChannel)
	}
	template, ok := templatesByKey[normalizedTemplateKey]
	if !ok {
		return ConnectionTemplate{}, fmt.Errorf("connection template not found: %s", normalizedTemplateKey)
	}

	rendered := copyConnectionTemplate(template)
	rendered.Title = applyTemplateParams(rendered.Title, params)
	rendered.Body = applyTemplateParams(rendered.Body, params)
	for i := range rendered.Buttons {
		rendered.Buttons[i].Label = applyTemplateParams(rendered.Buttons[i].Label, params)
		rendered.Buttons[i].Action = applyTemplateParams(rendered.Buttons[i].Action, params)
	}
	return rendered, nil
}

func (s *Service) UpsertFollowupRule(workspaceID string, rule FollowupRule) FollowupRule {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(workspaceID) == "" {
		workspaceID = "default"
	}
	s.ensureWorkspaceFollowupRulesLocked(workspaceID)
	if strings.TrimSpace(rule.RuleID) == "" {
		rule.RuleID = fmt.Sprintf("followup_rule_%06d", s.nextFollowupID)
		s.nextFollowupID++
	}
	rule.WorkspaceID = workspaceID
	if strings.TrimSpace(rule.Trigger) == "" {
		rule.Trigger = "onboarding_completed"
	}
	if strings.TrimSpace(rule.Question) == "" {
		rule.Question = "What is your highest-priority follow-up after onboarding?"
	}
	if strings.TrimSpace(rule.Status) == "" {
		rule.Status = "active"
	}
	s.followupRules[workspaceID][rule.RuleID] = rule
	return rule
}

func (s *Service) ListFollowupRules(workspaceID string) []FollowupRule {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(workspaceID) == "" {
		workspaceID = "default"
	}
	s.ensureWorkspaceFollowupRulesLocked(workspaceID)
	out := make([]FollowupRule, 0, len(s.followupRules[workspaceID]))
	for _, rule := range s.followupRules[workspaceID] {
		out = append(out, rule)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RuleID < out[j].RuleID
	})
	return out
}

func (s *Service) ListAdaptiveQuestions(workspaceID string) []AdaptiveQuestion {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(workspaceID) == "" {
		workspaceID = "default"
	}
	out := make([]AdaptiveQuestion, 0, len(s.adaptiveQuestions[workspaceID]))
	for _, question := range s.adaptiveQuestions[workspaceID] {
		out = append(out, question)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].FollowupID < out[j].FollowupID
	})
	return out
}

func (s *Service) AnswerAdaptiveQuestion(workspaceID, followupID, answer string) (AdaptiveQuestion, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(workspaceID) == "" {
		workspaceID = "default"
	}
	question, ok := s.adaptiveQuestions[workspaceID][followupID]
	if !ok {
		return AdaptiveQuestion{}, false, nil
	}
	if strings.TrimSpace(answer) == "" {
		return AdaptiveQuestion{}, false, fmt.Errorf("adaptive answer cannot be empty")
	}
	question.Answer = strings.TrimSpace(answer)
	question.Status = "answered"
	s.adaptiveQuestions[workspaceID][followupID] = question
	return question, true, nil
}

func (s *Service) ensureWorkspaceFollowupRulesLocked(workspaceID string) {
	if _, ok := s.followupRules[workspaceID]; !ok {
		s.followupRules[workspaceID] = map[string]FollowupRule{
			"default_rule_onboarding_completed": {
				RuleID:      "default_rule_onboarding_completed",
				WorkspaceID: workspaceID,
				Trigger:     "onboarding_completed",
				Question:    "What should the assistant prioritize in your first week?",
				Status:      "active",
			},
			"default_rule_meeting_load_high": {
				RuleID:      "default_rule_meeting_load_high",
				WorkspaceID: workspaceID,
				Trigger:     "meeting_load_high",
				Question:    "Should the assistant proactively prepare meeting briefs?",
				Status:      "active",
			},
			"default_rule_low_autonomy": {
				RuleID:      "default_rule_low_autonomy",
				WorkspaceID: workspaceID,
				Trigger:     "low_autonomy_preference",
				Question:    "Which actions should remain approval-gated at all times?",
				Status:      "active",
			},
		}
	}
	if _, ok := s.adaptiveQuestions[workspaceID]; !ok {
		s.adaptiveQuestions[workspaceID] = map[string]AdaptiveQuestion{}
	}
}

func (s *Service) generateAdaptiveQuestionsLocked(workspaceID string, operator, behavior, codebase, system map[string]string) {
	s.ensureWorkspaceFollowupRulesLocked(workspaceID)

	activeRules := []FollowupRule{}
	for _, rule := range s.followupRules[workspaceID] {
		if strings.ToLower(strings.TrimSpace(rule.Status)) != "active" {
			continue
		}
		if ruleApplies(rule.Trigger, operator, behavior, codebase, system) {
			activeRules = append(activeRules, rule)
		}
	}
	sort.Slice(activeRules, func(i, j int) bool {
		return activeRules[i].RuleID < activeRules[j].RuleID
	})

	for _, rule := range activeRules {
		if s.hasAdaptiveQuestionLocked(workspaceID, rule.Trigger) {
			continue
		}
		followupID := fmt.Sprintf("followup_%06d", s.nextFollowupID)
		s.nextFollowupID++
		s.adaptiveQuestions[workspaceID][followupID] = AdaptiveQuestion{
			FollowupID:  followupID,
			WorkspaceID: workspaceID,
			Question:    rule.Question,
			Trigger:     rule.Trigger,
			Status:      "pending",
			Answer:      "",
		}
	}
}

func (s *Service) hasAdaptiveQuestionLocked(workspaceID, trigger string) bool {
	for _, question := range s.adaptiveQuestions[workspaceID] {
		if question.Trigger == trigger {
			return true
		}
	}
	return false
}

func copyConnectionTemplate(template ConnectionTemplate) ConnectionTemplate {
	buttons := make([]OnboardingButton, len(template.Buttons))
	copy(buttons, template.Buttons)
	template.Buttons = buttons
	return template
}

func applyTemplateParams(value string, params map[string]string) string {
	if len(params) == 0 {
		return value
	}
	out := value
	for key, replacement := range params {
		placeholder := "{{" + strings.TrimSpace(key) + "}}"
		out = strings.ReplaceAll(out, placeholder, strings.TrimSpace(replacement))
	}
	return out
}

func ruleApplies(trigger string, operator, behavior, codebase, system map[string]string) bool {
	switch trigger {
	case "onboarding_completed":
		return true
	case "meeting_load_high":
		meetingLoad := strings.ToLower(strings.TrimSpace(codebase["meeting_load"]))
		return meetingLoad == "high" || meetingLoad == "very_high"
	case "low_autonomy_preference":
		autonomy := strings.ToUpper(strings.TrimSpace(behavior["autonomy_preference"]))
		return autonomy == "A0" || autonomy == "A1"
	default:
		_ = operator
		_ = system
		return false
	}
}
