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

var legacyQuestionAliases = map[string]map[string][]string{
	"operator_profile_intake_v1": {
		"OPI-001": {"role"},
		"OPI-002": {"role"},
		"OPI-003": {"timezone"},
		"OPI-004": {"notification_window"},
		"OPI-005": {"goals"},
		"OPI-006": {"integrations"},
		"OPI-007": {"communication_pref"},
		"OPI-008": {"goals"},
		"OPI-009": {"team_size"},
		"OPI-010": {"industry"},
	},
	"behavior_policy_calibration_v1": {
		"BPC-001": {"approval_threshold"},
		"BPC-002": {"autonomy_preference"},
		"BPC-003": {"risk_tolerance", "approval_threshold"},
		"BPC-004": {"proactive_mode"},
		"BPC-005": {"escalation_path"},
		"BPC-006": {"tone", "communication_pref"},
		"BPC-007": {"initiative_level"},
		"BPC-008": {"decision_style"},
	},
	"codebase_map_ingestion_v1": {
		"CBI-001": {"repo"},
		"CBI-002": {"stack"},
		"CBI-003": {"integrations"},
		"CBI-004": {"planning_horizon"},
		"CBI-005": {"delivery_cadence"},
	},
	"system_map_ingestion_v1": {
		"SMI-001": {"integrations"},
		"SMI-002": {"meeting_load", "sla"},
		"SMI-003": {"integrations"},
		"SMI-004": {"integrations"},
		"SMI-005": {"privacy_mode", "audit_strictness"},
	},
}

var defaultQuestionAnswers = map[string]map[string]string{
	"operator_profile_intake_v1": {
		"OPI-004": "09:00-17:00",
		"OPI-006": "google_calendar",
	},
	"behavior_policy_calibration_v1": {
		"BPC-005": "ask_each_time",
		"BPC-008": "ask_first",
	},
	"codebase_map_ingestion_v1": {
		"CBI-003": "kubernetes",
		"CBI-005": "github_actions",
	},
}

func NewService() *Service {
	return &Service{
		questionSets: map[string][]Question{
			"operator_profile_intake_v1": {
				{Key: "OPI-001", Prompt: "What is your full name?"},
				{Key: "OPI-002", Prompt: "What is your job title and company?"},
				{Key: "OPI-003", Prompt: "What timezone are you in?"},
				{Key: "OPI-004", Prompt: "What are your primary work hours?"},
				{Key: "OPI-005", Prompt: "What email accounts do you use for work?"},
				{Key: "OPI-006", Prompt: "What calendar systems do you use?"},
				{Key: "OPI-007", Prompt: "What messaging platforms do you use?"},
				{Key: "OPI-008", Prompt: "What task/project management tools do you use?"},
				{Key: "OPI-009", Prompt: "Do you manage a team? If so, how many people?"},
				{Key: "OPI-010", Prompt: "What is your primary industry or domain?"},
			},
			"behavior_policy_calibration_v1": {
				{Key: "BPC-001", Prompt: "How would you like me to handle scheduling conflicts? (Ask me / Suggest best option / Auto-resolve)"},
				{Key: "BPC-002", Prompt: "For sending emails on your behalf, should I always ask for approval, or can I send routine responses automatically?"},
				{Key: "BPC-003", Prompt: "How should I handle financial transactions? (Always confirm / Auto-approve under $X / Never act)"},
				{Key: "BPC-004", Prompt: "When you receive an urgent message, should I proactively notify you or wait for you to check?"},
				{Key: "BPC-005", Prompt: "How should I handle requests from your team members? (Full access / Read-only / Ask me each time)"},
				{Key: "BPC-006", Prompt: "What's your preferred communication style? (Brief / Detailed / Formal / Casual)"},
				{Key: "BPC-007", Prompt: "Should I learn from your patterns over time and adjust my behavior?"},
				{Key: "BPC-008", Prompt: "How should I handle unknown or new types of requests? (Ask first / Try and report / Refuse)"},
			},
			"codebase_map_ingestion_v1": {
				{Key: "CBI-001", Prompt: "Do you work with any code repositories? If so, provide the HTTPS URL(s)."},
				{Key: "CBI-002", Prompt: "What is the primary programming language?"},
				{Key: "CBI-003", Prompt: "What deployment platform do you use?"},
				{Key: "CBI-004", Prompt: "Describe your branching strategy."},
				{Key: "CBI-005", Prompt: "Are there any CI/CD pipelines I should know about?"},
			},
			"system_map_ingestion_v1": {
				{Key: "SMI-001", Prompt: "What cloud providers or infrastructure do you use?"},
				{Key: "SMI-002", Prompt: "What monitoring or alerting tools do you use?"},
				{Key: "SMI-003", Prompt: "What internal tools or custom systems does your team use?"},
				{Key: "SMI-004", Prompt: "Are there any API integrations you'd like me to connect to that we haven't discussed?"},
				{Key: "SMI-005", Prompt: "Any security policies or compliance requirements I should be aware of?"},
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
		if strings.TrimSpace(resolveAnswer(stageKey, q.Key, answers)) == "" {
			return StageResult{}, fmt.Errorf("missing answer for %s", q.Key)
		}
	}

	key := replayKey(workspaceID, stageKey, answers)
	if cached, ok := s.replay[key]; ok {
		return StageResult{StageKey: stageKey, Extracted: copyStringMap(cached)}, nil
	}

	extracted := map[string]string{}
	for _, q := range questions {
		extracted[q.Key] = strings.TrimSpace(resolveAnswer(stageKey, q.Key, answers))
	}
	s.replay[key] = copyStringMap(extracted)
	return StageResult{StageKey: stageKey, Extracted: extracted}, nil
}

func resolveAnswer(stageKey, key string, answers map[string]string) string {
	if value := strings.TrimSpace(answers[key]); value != "" {
		return value
	}
	aliases := legacyQuestionAliases[stageKey][key]
	for _, alias := range aliases {
		if value := strings.TrimSpace(answers[alias]); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(defaultQuestionAnswers[stageKey][key]); value != "" {
		return value
	}
	return ""
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
			"role":                op["OPI-002"],
			"goals":               cb["CBI-005"],
			"industry":            op["OPI-010"],
			"team_size":           op["OPI-009"],
			"timezone":            op["OPI-003"],
			"decision_style":      bp["BPC-008"],
			"communication_pref":  bp["BPC-006"],
			"kpi_primary":         sy["SMI-002"],
			"risk_tolerance":      bp["BPC-003"],
			"autonomy_preference": bp["BPC-002"],
			"planning_horizon":    cb["CBI-004"],
			"meeting_load":        sy["SMI-002"],
			"focus_mode":          bp["BPC-007"],
		},
	}
	s.personas[workspaceID] = WorkspacePersona{
		WorkspaceID: workspaceID,
		VersionInt:  personaVersion,
		Persona: map[string]string{
			"tone":               bp["BPC-006"],
			"initiative_level":   bp["BPC-004"],
			"language":           op["OPI-003"],
			"communication_pref": bp["BPC-006"],
			"decision_style":     bp["BPC-008"],
		},
	}
	s.behaviorPolicies[workspaceID] = WorkspaceBehaviorPolicy{
		WorkspaceID: workspaceID,
		VersionInt:  policyVersion,
		Policy: map[string]string{
			"approval_threshold":  bp["BPC-003"],
			"proactive_mode":      bp["BPC-004"],
			"notification_window": op["OPI-004"],
			"write_actions":       bp["BPC-002"],
			"escalation_path":     bp["BPC-004"],
			"privacy_mode":        sy["SMI-005"],
			"audit_strictness":    sy["SMI-005"],
			"delivery_cadence":    cb["CBI-005"],
			"context_budget":      sy["SMI-001"],
			"sla":                 sy["SMI-002"],
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
		meetingLoad := strings.ToLower(strings.TrimSpace(system["SMI-002"]))
		return meetingLoad == "high" || meetingLoad == "very_high"
	case "low_autonomy_preference":
		emailAutonomy := strings.ToLower(strings.TrimSpace(behavior["BPC-002"]))
		return strings.Contains(emailAutonomy, "always ask") || strings.Contains(emailAutonomy, "ask")
	default:
		_ = operator
		_ = system
		return false
	}
}
