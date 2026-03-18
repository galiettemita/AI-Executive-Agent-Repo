package identity

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/brevio/brevio/internal/audit"
	"github.com/google/uuid"

	"github.com/brevio/brevio/internal/determinism"
)

type Account struct {
	ID                 uuid.UUID
	PlanKey            string
	Status             string
	BillingCustomerRef string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type User struct {
	ID             uuid.UUID
	AccountID      uuid.UUID
	Email          string
	PhoneE164      string
	GlobalAutonomy string
	Timezone       string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Workspace struct {
	ID                   uuid.UUID
	AccountID            uuid.UUID
	OwnerUserID          uuid.UUID
	Status               string
	MemoryNamespace      string
	DomainAutonomyJSON   string
	AllowedConnectorKeys []string
	Region               string // "us-east-1" or "eu-west-1"; default "us-east-1"
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type ChannelBinding struct {
	ID                uuid.UUID
	WorkspaceID       uuid.UUID
	Channel           string
	ChannelIdentifier string
	CreatedAt         time.Time
}

type Service struct {
	mu                   sync.RWMutex
	accounts             map[uuid.UUID]Account
	users                map[uuid.UUID]User
	workspaces           map[uuid.UUID]Workspace
	channelBindingLookup map[string]uuid.UUID
	mutationAudit        *audit.Service
}

type AccountUpdate struct {
	PlanKey            *string
	Status             *string
	BillingCustomerRef *string
}

type UserUpdate struct {
	Email          *string
	PhoneE164      *string
	GlobalAutonomy *string
	Timezone       *string
	Status         *string
}

var defaultDomainAutonomy = map[string]string{
	"calendar":    "A2",
	"email":       "A1",
	"messaging":   "A1",
	"tasks":       "A2",
	"documents":   "A1",
	"crm":         "A1",
	"travel":      "A2",
	"financial":   "A1",
	"health":      "A0",
	"environment": "A1",
	"web":         "A3",
}

func NewService() *Service {
	return &Service{
		accounts:             map[uuid.UUID]Account{},
		users:                map[uuid.UUID]User{},
		workspaces:           map[uuid.UUID]Workspace{},
		channelBindingLookup: map[string]uuid.UUID{},
	}
}

func (s *Service) SetMutationAudit(auditSvc *audit.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mutationAudit = auditSvc
}

func (s *Service) CreateAccount(planKey, status, billingRef string) (Account, error) {
	if planKey == "" {
		return Account{}, fmt.Errorf("plan_key is required")
	}
	id, err := determinism.NewUUIDv7()
	if err != nil {
		return Account{}, err
	}
	now := time.Now().UTC()
	account := Account{
		ID:                 id,
		PlanKey:            planKey,
		Status:             status,
		BillingCustomerRef: billingRef,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if account.Status == "" {
		account.Status = "active"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[account.ID] = account
	return account, nil
}

func (s *Service) GetAccount(accountID uuid.UUID) (Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	account, ok := s.accounts[accountID]
	if !ok {
		return Account{}, fmt.Errorf("account not found")
	}
	return account, nil
}

func (s *Service) UpdateAccount(accountID uuid.UUID, update AccountUpdate) (Account, error) {
	if accountID == uuid.Nil {
		return Account{}, fmt.Errorf("account_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	account, ok := s.accounts[accountID]
	if !ok {
		return Account{}, fmt.Errorf("account not found")
	}
	if update.PlanKey != nil && *update.PlanKey != "" {
		account.PlanKey = *update.PlanKey
	}
	if update.Status != nil && *update.Status != "" {
		account.Status = *update.Status
	}
	if update.BillingCustomerRef != nil {
		account.BillingCustomerRef = *update.BillingCustomerRef
	}
	account.UpdatedAt = time.Now().UTC()
	s.accounts[accountID] = account
	return account, nil
}

func (s *Service) CreateUser(accountID uuid.UUID, email, phoneE164, globalAutonomy, timezone string) (User, error) {
	if accountID == uuid.Nil {
		return User{}, fmt.Errorf("account_id is required")
	}
	if email == "" {
		return User{}, fmt.Errorf("email is required")
	}
	if timezone == "" {
		timezone = "UTC"
	}
	if globalAutonomy == "" {
		globalAutonomy = "A1"
	}

	id, err := determinism.NewUUIDv7()
	if err != nil {
		return User{}, err
	}
	now := time.Now().UTC()
	user := User{
		ID:             id,
		AccountID:      accountID,
		Email:          email,
		PhoneE164:      phoneE164,
		GlobalAutonomy: globalAutonomy,
		Timezone:       timezone,
		Status:         "active",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.accounts[accountID]; !ok {
		return User{}, fmt.Errorf("account not found")
	}
	s.users[user.ID] = user
	return user, nil
}

func (s *Service) GetUser(userID uuid.UUID) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[userID]
	if !ok {
		return User{}, fmt.Errorf("user not found")
	}
	return user, nil
}

func (s *Service) UpdateUser(userID uuid.UUID, update UserUpdate) (User, error) {
	if userID == uuid.Nil {
		return User{}, fmt.Errorf("user_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[userID]
	if !ok {
		return User{}, fmt.Errorf("user not found")
	}
	before := user
	if update.Email != nil && *update.Email != "" {
		user.Email = *update.Email
	}
	if update.PhoneE164 != nil {
		user.PhoneE164 = *update.PhoneE164
	}
	if update.GlobalAutonomy != nil && *update.GlobalAutonomy != "" {
		user.GlobalAutonomy = *update.GlobalAutonomy
	}
	if update.Timezone != nil && *update.Timezone != "" {
		user.Timezone = *update.Timezone
	}
	if update.Status != nil && *update.Status != "" {
		user.Status = *update.Status
	}
	user.UpdatedAt = time.Now().UTC()
	s.users[userID] = user
	mutationAudit := s.mutationAudit
	if mutationAudit != nil {
		mutationAudit.AppendMutation(audit.MutationInput{
			WorkspaceID: user.AccountID.String(),
			Actor:       user.ID.String(),
			Action:      "identity.user.profile.update",
			Resource:    "user:" + user.ID.String(),
			Before: map[string]any{
				"email":           before.Email,
				"phone_e164":      before.PhoneE164,
				"global_autonomy": before.GlobalAutonomy,
				"timezone":        before.Timezone,
				"status":          before.Status,
			},
			After: map[string]any{
				"email":           user.Email,
				"phone_e164":      user.PhoneE164,
				"global_autonomy": user.GlobalAutonomy,
				"timezone":        user.Timezone,
				"status":          user.Status,
			},
		})
	}
	return user, nil
}

func (s *Service) DeleteUser(userID uuid.UUID) error {
	if userID == uuid.Nil {
		return fmt.Errorf("user_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}
	user.Status = "deleted"
	user.UpdatedAt = time.Now().UTC()
	s.users[userID] = user
	return nil
}

func (s *Service) CreateWorkspace(accountID, ownerUserID uuid.UUID, memoryNamespace, domainAutonomyJSON string, allowedConnectorKeys []string) (Workspace, error) {
	if accountID == uuid.Nil || ownerUserID == uuid.Nil {
		return Workspace{}, fmt.Errorf("account_id and owner_user_id are required")
	}
	if memoryNamespace == "" {
		return Workspace{}, fmt.Errorf("memory_namespace is required")
	}
	if domainAutonomyJSON == "" {
		encodedDefaults, err := json.Marshal(defaultDomainAutonomy)
		if err != nil {
			return Workspace{}, fmt.Errorf("encode default domain autonomy: %w", err)
		}
		domainAutonomyJSON = string(encodedDefaults)
	}

	id, err := determinism.NewUUIDv7()
	if err != nil {
		return Workspace{}, err
	}
	now := time.Now().UTC()
	workspace := Workspace{
		ID:                   id,
		AccountID:            accountID,
		OwnerUserID:          ownerUserID,
		Status:               "active",
		MemoryNamespace:      memoryNamespace,
		DomainAutonomyJSON:   domainAutonomyJSON,
		AllowedConnectorKeys: allowedConnectorKeys,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.accounts[accountID]; !ok {
		return Workspace{}, fmt.Errorf("account not found")
	}
	if _, ok := s.users[ownerUserID]; !ok {
		return Workspace{}, fmt.Errorf("owner user not found")
	}
	s.workspaces[workspace.ID] = workspace
	return workspace, nil
}

func (s *Service) ArchiveWorkspace(workspaceID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	workspace, ok := s.workspaces[workspaceID]
	if !ok {
		return fmt.Errorf("workspace not found")
	}
	workspace.Status = "archived"
	workspace.UpdatedAt = time.Now().UTC()
	s.workspaces[workspaceID] = workspace
	return nil
}

func (s *Service) GetWorkspace(workspaceID uuid.UUID) (Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspace, ok := s.workspaces[workspaceID]
	if !ok {
		return Workspace{}, fmt.Errorf("workspace not found")
	}
	return workspace, nil
}

func bindingKey(channel, identifier string) string {
	return channel + "::" + identifier
}

func (s *Service) BindChannel(workspaceID uuid.UUID, channel, identifier string) (ChannelBinding, error) {
	if workspaceID == uuid.Nil {
		return ChannelBinding{}, fmt.Errorf("workspace_id is required")
	}
	if channel == "" || identifier == "" {
		return ChannelBinding{}, fmt.Errorf("channel and identifier are required")
	}
	if !isSupportedChannel(channel) {
		return ChannelBinding{}, fmt.Errorf("unsupported channel: %s", channel)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[workspaceID]; !ok {
		return ChannelBinding{}, fmt.Errorf("workspace not found")
	}

	lookup := bindingKey(channel, identifier)
	if _, exists := s.channelBindingLookup[lookup]; exists {
		return ChannelBinding{}, fmt.Errorf("channel binding already exists")
	}

	id, err := determinism.NewUUIDv7()
	if err != nil {
		return ChannelBinding{}, err
	}
	binding := ChannelBinding{
		ID:                id,
		WorkspaceID:       workspaceID,
		Channel:           channel,
		ChannelIdentifier: identifier,
		CreatedAt:         time.Now().UTC(),
	}
	s.channelBindingLookup[lookup] = workspaceID
	return binding, nil
}

func isSupportedChannel(channel string) bool {
	switch channel {
	case "whatsapp", "imessage":
		return true
	default:
		return false
	}
}

func (s *Service) ResolveWorkspaceByChannel(channel, identifier string) (uuid.UUID, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspaceID, ok := s.channelBindingLookup[bindingKey(channel, identifier)]
	if !ok {
		return uuid.Nil, fmt.Errorf("workspace not found for channel binding")
	}
	return workspaceID, nil
}
