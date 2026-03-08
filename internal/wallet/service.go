package wallet

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Wallet represents a digital wallet for a workspace.
type Wallet struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	BalanceCents int64    `json:"balance_cents"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"` // active, frozen, closed
	CreatedAt   time.Time `json:"created_at"`
}

// Transaction represents a wallet transaction.
type Transaction struct {
	ID          string    `json:"id"`
	WalletID    string    `json:"wallet_id"`
	AmountCents int64     `json:"amount_cents"`
	Type        string    `json:"type"` // credit, debit
	Description string    `json:"description"`
	MerchantID  string    `json:"merchant_id"`
	Status      string    `json:"status"` // pending, approved, completed, reversed
	ApprovedBy  string    `json:"approved_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// Merchant represents an approved merchant.
type Merchant struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Category            string `json:"category"`
	Approved            bool   `json:"approved"`
	MaxTransactionCents int64  `json:"max_transaction_cents"`
}

// WalletService manages digital wallets and autonomous spending.
type WalletService struct {
	mu          sync.Mutex
	wallets     map[string]Wallet
	transactions map[string]Transaction
	merchants   map[string]Merchant
	now         func() time.Time
}

// NewWalletService creates a new WalletService.
func NewWalletService() *WalletService {
	return &WalletService{
		wallets:      map[string]Wallet{},
		transactions: map[string]Transaction{},
		merchants:    map[string]Merchant{},
		now:          func() time.Time { return time.Now().UTC() },
	}
}

// CreateWallet creates a new wallet for a workspace.
func (s *WalletService) CreateWallet(workspaceID string, currency string) (*Wallet, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if currency == "" {
		currency = "USD"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for existing active wallet.
	for _, w := range s.wallets {
		if w.WorkspaceID == workspaceID && w.Status == "active" {
			return nil, fmt.Errorf("workspace %s already has an active wallet", workspaceID)
		}
	}

	wallet := Wallet{
		ID:           uuid.Must(uuid.NewV7()).String(),
		WorkspaceID:  workspaceID,
		BalanceCents: 0,
		Currency:     currency,
		Status:       "active",
		CreatedAt:    s.now(),
	}
	s.wallets[wallet.ID] = wallet
	return &wallet, nil
}

// Fund adds funds to a wallet.
func (s *WalletService) Fund(walletID string, amountCents int64) error {
	if amountCents <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	wallet, ok := s.wallets[walletID]
	if !ok {
		return fmt.Errorf("wallet not found: %s", walletID)
	}
	if wallet.Status != "active" {
		return fmt.Errorf("wallet is not active: %s", wallet.Status)
	}

	wallet.BalanceCents += amountCents
	s.wallets[walletID] = wallet

	tx := Transaction{
		ID:          uuid.Must(uuid.NewV7()).String(),
		WalletID:    walletID,
		AmountCents: amountCents,
		Type:        "credit",
		Description: "Wallet funding",
		Status:      "completed",
		CreatedAt:   s.now(),
	}
	s.transactions[tx.ID] = tx
	return nil
}

// Debit creates a debit transaction, checking balance, merchant approval, and limits.
func (s *WalletService) Debit(walletID string, amountCents int64, merchantID, description string) (*Transaction, error) {
	if amountCents <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	wallet, ok := s.wallets[walletID]
	if !ok {
		return nil, fmt.Errorf("wallet not found: %s", walletID)
	}
	if wallet.Status != "active" {
		return nil, fmt.Errorf("wallet is not active: %s", wallet.Status)
	}
	if wallet.BalanceCents < amountCents {
		return nil, fmt.Errorf("insufficient balance: have %d, need %d", wallet.BalanceCents, amountCents)
	}

	if merchantID != "" {
		merchant, mok := s.merchants[merchantID]
		if !mok {
			return nil, fmt.Errorf("merchant not found: %s", merchantID)
		}
		if !merchant.Approved {
			return nil, fmt.Errorf("merchant %s is not approved", merchantID)
		}
		if merchant.MaxTransactionCents > 0 && amountCents > merchant.MaxTransactionCents {
			return nil, fmt.Errorf("amount %d exceeds merchant limit %d", amountCents, merchant.MaxTransactionCents)
		}
	}

	wallet.BalanceCents -= amountCents
	s.wallets[walletID] = wallet

	tx := Transaction{
		ID:          uuid.Must(uuid.NewV7()).String(),
		WalletID:    walletID,
		AmountCents: amountCents,
		Type:        "debit",
		Description: description,
		MerchantID:  merchantID,
		Status:      "pending",
		CreatedAt:   s.now(),
	}
	s.transactions[tx.ID] = tx
	return &tx, nil
}

// ApproveTransaction approves a pending transaction.
func (s *WalletService) ApproveTransaction(txID, approvedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, ok := s.transactions[txID]
	if !ok {
		return fmt.Errorf("transaction not found: %s", txID)
	}
	if tx.Status != "pending" {
		return fmt.Errorf("transaction is not pending: %s", tx.Status)
	}
	tx.Status = "approved"
	tx.ApprovedBy = approvedBy
	s.transactions[txID] = tx
	return nil
}

// ReverseTransaction reverses a completed or approved transaction.
func (s *WalletService) ReverseTransaction(txID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, ok := s.transactions[txID]
	if !ok {
		return fmt.Errorf("transaction not found: %s", txID)
	}
	if tx.Status == "reversed" {
		return fmt.Errorf("transaction already reversed")
	}
	if tx.Type != "debit" {
		return fmt.Errorf("can only reverse debit transactions")
	}

	wallet, wok := s.wallets[tx.WalletID]
	if !wok {
		return fmt.Errorf("wallet not found: %s", tx.WalletID)
	}

	wallet.BalanceCents += tx.AmountCents
	s.wallets[tx.WalletID] = wallet

	tx.Status = "reversed"
	s.transactions[txID] = tx
	return nil
}

// GetBalance returns the current balance in cents.
func (s *WalletService) GetBalance(walletID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet, ok := s.wallets[walletID]
	if !ok {
		return 0, fmt.Errorf("wallet not found: %s", walletID)
	}
	return wallet.BalanceCents, nil
}

// RegisterMerchant registers a new merchant.
func (s *WalletService) RegisterMerchant(name, category string, maxCents int64) (*Merchant, error) {
	if name == "" {
		return nil, fmt.Errorf("merchant name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	merchant := Merchant{
		ID:                  uuid.Must(uuid.NewV7()).String(),
		Name:                name,
		Category:            category,
		Approved:            true,
		MaxTransactionCents: maxCents,
	}
	s.merchants[merchant.ID] = merchant
	return &merchant, nil
}

// ListMerchants returns all registered merchants.
func (s *WalletService) ListMerchants() []Merchant {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Merchant, 0, len(s.merchants))
	for _, m := range s.merchants {
		out = append(out, m)
	}
	return out
}

// ListTransactions returns all transactions for a wallet.
func (s *WalletService) ListTransactions(walletID string) []Transaction {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []Transaction
	for _, tx := range s.transactions {
		if tx.WalletID == walletID {
			out = append(out, tx)
		}
	}
	return out
}
