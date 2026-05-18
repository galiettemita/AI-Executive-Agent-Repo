package wallet

import "testing"

func TestCreateWallet(t *testing.T) {
	t.Parallel()
	s := NewWalletService()

	w, err := s.CreateWallet("ws1", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.BalanceCents != 0 {
		t.Fatalf("expected 0 balance, got %d", w.BalanceCents)
	}
	if w.Status != "active" {
		t.Fatalf("expected active status, got %s", w.Status)
	}
}

func TestCreateWalletDuplicate(t *testing.T) {
	t.Parallel()
	s := NewWalletService()

	_, _ = s.CreateWallet("ws1", "USD")
	_, err := s.CreateWallet("ws1", "USD")
	if err == nil {
		t.Fatal("expected error for duplicate wallet")
	}
}

func TestFundWallet(t *testing.T) {
	t.Parallel()
	s := NewWalletService()

	w, _ := s.CreateWallet("ws1", "USD")
	if err := s.Fund(w.ID, 10000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	balance, _ := s.GetBalance(w.ID)
	if balance != 10000 {
		t.Fatalf("expected 10000 balance, got %d", balance)
	}
}

func TestFundNegativeAmount(t *testing.T) {
	t.Parallel()
	s := NewWalletService()

	w, _ := s.CreateWallet("ws1", "USD")
	err := s.Fund(w.ID, -100)
	if err == nil {
		t.Fatal("expected error for negative amount")
	}
}

func TestDebitSuccess(t *testing.T) {
	t.Parallel()
	s := NewWalletService()

	w, _ := s.CreateWallet("ws1", "USD")
	_ = s.Fund(w.ID, 10000)

	tx, err := s.Debit(w.ID, 3000, "", "Test debit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Status != "pending" {
		t.Fatalf("expected pending status, got %s", tx.Status)
	}

	balance, _ := s.GetBalance(w.ID)
	if balance != 7000 {
		t.Fatalf("expected 7000 balance, got %d", balance)
	}
}

func TestDebitInsufficientBalance(t *testing.T) {
	t.Parallel()
	s := NewWalletService()

	w, _ := s.CreateWallet("ws1", "USD")
	_ = s.Fund(w.ID, 1000)

	_, err := s.Debit(w.ID, 5000, "", "Over-spend")
	if err == nil {
		t.Fatal("expected error for insufficient balance")
	}
}

func TestDebitWithMerchantLimit(t *testing.T) {
	t.Parallel()
	s := NewWalletService()

	w, _ := s.CreateWallet("ws1", "USD")
	_ = s.Fund(w.ID, 100000)
	m, _ := s.RegisterMerchant("TestCo", "software", 5000)

	_, err := s.Debit(w.ID, 10000, m.ID, "Over limit")
	if err == nil {
		t.Fatal("expected error for exceeding merchant limit")
	}

	tx, err := s.Debit(w.ID, 3000, m.ID, "Within limit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.MerchantID != m.ID {
		t.Fatalf("expected merchant ID %s, got %s", m.ID, tx.MerchantID)
	}
}

func TestApproveTransaction(t *testing.T) {
	t.Parallel()
	s := NewWalletService()

	w, _ := s.CreateWallet("ws1", "USD")
	_ = s.Fund(w.ID, 10000)
	tx, _ := s.Debit(w.ID, 1000, "", "Test")

	if err := s.ApproveTransaction(tx.ID, "admin"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReverseTransaction(t *testing.T) {
	t.Parallel()
	s := NewWalletService()

	w, _ := s.CreateWallet("ws1", "USD")
	_ = s.Fund(w.ID, 10000)
	tx, _ := s.Debit(w.ID, 3000, "", "Test")

	if err := s.ReverseTransaction(tx.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	balance, _ := s.GetBalance(w.ID)
	if balance != 10000 {
		t.Fatalf("expected 10000 after reversal, got %d", balance)
	}
}

func TestReverseAlreadyReversed(t *testing.T) {
	t.Parallel()
	s := NewWalletService()

	w, _ := s.CreateWallet("ws1", "USD")
	_ = s.Fund(w.ID, 10000)
	tx, _ := s.Debit(w.ID, 1000, "", "Test")
	_ = s.ReverseTransaction(tx.ID)

	err := s.ReverseTransaction(tx.ID)
	if err == nil {
		t.Fatal("expected error reversing already reversed transaction")
	}
}

func TestRegisterMerchant(t *testing.T) {
	t.Parallel()
	s := NewWalletService()

	m, err := s.RegisterMerchant("AcmeCo", "tools", 50000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !m.Approved {
		t.Fatal("expected merchant to be approved")
	}
}

func TestListTransactions(t *testing.T) {
	t.Parallel()
	s := NewWalletService()

	w, _ := s.CreateWallet("ws1", "USD")
	_ = s.Fund(w.ID, 10000)
	_, _ = s.Debit(w.ID, 1000, "", "Test1")
	_, _ = s.Debit(w.ID, 2000, "", "Test2")

	txs := s.ListTransactions(w.ID)
	if len(txs) != 3 { // 1 credit + 2 debits
		t.Fatalf("expected 3 transactions, got %d", len(txs))
	}
}

func TestGetBalanceNotFound(t *testing.T) {
	t.Parallel()
	s := NewWalletService()

	_, err := s.GetBalance("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing wallet")
	}
}
