package provisioning

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Plan tiers.
const (
	PlanFree       = "free"
	PlanStarter    = "starter"
	PlanPro        = "pro"
	PlanEnterprise = "enterprise"
)

// Risk levels for packages.
const (
	RiskLow      = "low"
	RiskMedium   = "medium"
	RiskElevated = "elevated"
)

// Provisioning request statuses.
const (
	ProvisionStatusPending    = "pending"
	ProvisionStatusInProgress = "in_progress"
	ProvisionStatusCompleted  = "completed"
	ProvisionStatusFailed     = "failed"
)

// PackageDefinition describes a provisionable connector package.
type PackageDefinition struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	RequiredPlanTier   string   `json:"required_plan_tier"`
	MinTrustScore      float64  `json:"min_trust_score"`
	RiskLevel          string   `json:"risk_level"`
	RequiredConnectors []string `json:"required_connectors"`
	OAuthScopes        []string `json:"oauth_scopes"`
}

// EligibilityResult describes whether a workspace can provision a package.
type EligibilityResult struct {
	Eligible    bool   `json:"eligible"`
	PackageID   string `json:"package_id"`
	WorkspaceID string `json:"workspace_id"`
	Reason      string `json:"reason,omitempty"`
}

// ProvisioningRequest tracks an in-flight provisioning operation.
type ProvisioningRequest struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	PackageID   string    `json:"package_id"`
	Status      string    `json:"status"`
	FailReason  string    `json:"fail_reason,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

var packageRegistry = map[string]PackageDefinition{
	"package_a": {
		ID:                 "package_a",
		Name:               "Core Messaging",
		Description:        "WhatsApp and iMessage connectors for unified messaging",
		RequiredPlanTier:   PlanStarter,
		MinTrustScore:      0.3,
		RiskLevel:          RiskLow,
		RequiredConnectors: []string{"whatsapp", "imessage"},
		OAuthScopes:        []string{"messaging.read", "messaging.send"},
	},
	"package_b": {
		ID:                 "package_b",
		Name:               "MCP Servers",
		Description:        "Third-party MCP tool server integrations",
		RequiredPlanTier:   PlanPro,
		MinTrustScore:      0.5,
		RiskLevel:          RiskMedium,
		RequiredConnectors: []string{"mcp_registry", "mcp_runtime"},
		OAuthScopes:        []string{"mcp.discover", "mcp.invoke"},
	},
	"package_c": {
		ID:                 "package_c",
		Name:               "Calendar & Productivity",
		Description:        "Calendar, task management, and productivity tool connectors",
		RequiredPlanTier:   PlanStarter,
		MinTrustScore:      0.3,
		RiskLevel:          RiskLow,
		RequiredConnectors: []string{"google_calendar", "outlook_calendar", "todoist"},
		OAuthScopes:        []string{"calendar.read", "calendar.write", "tasks.manage"},
	},
	"package_d": {
		ID:                 "package_d",
		Name:               "Financial Tools",
		Description:        "Financial data connectors with elevated risk controls",
		RequiredPlanTier:   PlanEnterprise,
		MinTrustScore:      0.8,
		RiskLevel:          RiskElevated,
		RequiredConnectors: []string{"plaid", "stripe", "quickbooks"},
		OAuthScopes:        []string{"finance.read", "finance.transactions", "finance.accounts"},
	},
	"package_e": {
		ID:                 "package_e",
		Name:               "Smart Home / IoT",
		Description:        "Smart home and IoT device connectors",
		RequiredPlanTier:   PlanPro,
		MinTrustScore:      0.5,
		RiskLevel:          RiskMedium,
		RequiredConnectors: []string{"homekit", "smartthings", "mqtt_bridge"},
		OAuthScopes:        []string{"iot.read", "iot.control", "iot.automate"},
	},
	"package_f": {
		ID:                 "package_f",
		Name:               "Developer Tools",
		Description:        "Git, CI/CD, and developer workflow integrations",
		RequiredPlanTier:   PlanPro,
		MinTrustScore:      0.6,
		RiskLevel:          RiskMedium,
		RequiredConnectors: []string{"github", "gitlab", "circleci"},
		OAuthScopes:        []string{"repo.read", "repo.write", "ci.trigger", "ci.status"},
	},
}

// planRank returns a numeric rank for plan comparison.
func planRank(plan string) int {
	switch plan {
	case PlanFree:
		return 0
	case PlanStarter:
		return 1
	case PlanPro:
		return 2
	case PlanEnterprise:
		return 3
	default:
		return -1
	}
}

// ProvisioningEngine manages auto-provisioning of connector packages.
type ProvisioningEngine struct {
	mu       sync.Mutex
	requests map[string]*ProvisioningRequest            // requestID -> request
	byWS     map[string][]string                        // workspaceID -> []requestID
	now      func() time.Time
}

// NewProvisioningEngine creates a new provisioning engine.
func NewProvisioningEngine() *ProvisioningEngine {
	return &ProvisioningEngine{
		requests: map[string]*ProvisioningRequest{},
		byWS:     map[string][]string{},
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// ResolvePackage returns the package definition for the given ID.
func (e *ProvisioningEngine) ResolvePackage(packageID string) (*PackageDefinition, error) {
	pkg, ok := packageRegistry[packageID]
	if !ok {
		return nil, fmt.Errorf("unknown package: %s", packageID)
	}
	cp := pkg
	connectors := make([]string, len(pkg.RequiredConnectors))
	copy(connectors, pkg.RequiredConnectors)
	cp.RequiredConnectors = connectors
	scopes := make([]string, len(pkg.OAuthScopes))
	copy(scopes, pkg.OAuthScopes)
	cp.OAuthScopes = scopes
	return &cp, nil
}

// EvaluateEligibility checks whether a workspace is eligible to provision the package.
func (e *ProvisioningEngine) EvaluateEligibility(workspaceID, packageID string, trustScore float64, plan string) (*EligibilityResult, error) {
	pkg, ok := packageRegistry[packageID]
	if !ok {
		return nil, fmt.Errorf("unknown package: %s", packageID)
	}

	result := &EligibilityResult{
		PackageID:   packageID,
		WorkspaceID: workspaceID,
	}

	requiredRank := planRank(pkg.RequiredPlanTier)
	actualRank := planRank(plan)
	if actualRank < 0 {
		result.Eligible = false
		result.Reason = fmt.Sprintf("unknown plan: %s", plan)
		return result, nil
	}
	if actualRank < requiredRank {
		result.Eligible = false
		result.Reason = fmt.Sprintf("plan %s does not meet required tier %s", plan, pkg.RequiredPlanTier)
		return result, nil
	}

	if trustScore < pkg.MinTrustScore {
		result.Eligible = false
		result.Reason = fmt.Sprintf("trust score %.2f below minimum %.2f", trustScore, pkg.MinTrustScore)
		return result, nil
	}

	result.Eligible = true
	return result, nil
}

// StartProvisioning creates a new provisioning request for the given workspace and package.
func (e *ProvisioningEngine) StartProvisioning(workspaceID, packageID string) (*ProvisioningRequest, error) {
	if _, ok := packageRegistry[packageID]; !ok {
		return nil, fmt.Errorf("unknown package: %s", packageID)
	}
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID must not be empty")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Check for duplicate in-progress requests.
	for _, reqID := range e.byWS[workspaceID] {
		req := e.requests[reqID]
		if req.PackageID == packageID && (req.Status == ProvisionStatusPending || req.Status == ProvisionStatusInProgress) {
			return nil, fmt.Errorf("provisioning already in progress for package %s in workspace %s", packageID, workspaceID)
		}
	}

	now := e.now()
	req := &ProvisioningRequest{
		ID:          uuid.Must(uuid.NewV7()).String(),
		WorkspaceID: workspaceID,
		PackageID:   packageID,
		Status:      ProvisionStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	e.requests[req.ID] = req
	e.byWS[workspaceID] = append(e.byWS[workspaceID], req.ID)

	cp := *req
	return &cp, nil
}

// CompleteProvisioning marks a provisioning request as completed.
func (e *ProvisioningEngine) CompleteProvisioning(requestID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	req, ok := e.requests[requestID]
	if !ok {
		return fmt.Errorf("provisioning request not found: %s", requestID)
	}
	if req.Status != ProvisionStatusPending && req.Status != ProvisionStatusInProgress {
		return fmt.Errorf("cannot complete request in status %s", req.Status)
	}

	req.Status = ProvisionStatusCompleted
	req.UpdatedAt = e.now()
	return nil
}

// FailProvisioning marks a provisioning request as failed with a reason.
func (e *ProvisioningEngine) FailProvisioning(requestID string, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	req, ok := e.requests[requestID]
	if !ok {
		return fmt.Errorf("provisioning request not found: %s", requestID)
	}
	if req.Status != ProvisionStatusPending && req.Status != ProvisionStatusInProgress {
		return fmt.Errorf("cannot fail request in status %s", req.Status)
	}

	req.Status = ProvisionStatusFailed
	req.FailReason = reason
	req.UpdatedAt = e.now()
	return nil
}

// ListRequests returns all provisioning requests for the given workspace.
func (e *ProvisioningEngine) ListRequests(workspaceID string) []ProvisioningRequest {
	e.mu.Lock()
	defer e.mu.Unlock()

	ids := e.byWS[workspaceID]
	out := make([]ProvisioningRequest, 0, len(ids))
	for _, id := range ids {
		if req, ok := e.requests[id]; ok {
			out = append(out, *req)
		}
	}
	return out
}
