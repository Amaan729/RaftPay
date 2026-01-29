package ledger

import (
	"fmt"
	"sync"
)

// Account represents a financial account
type Account struct {
	ID      string
	Balance float64
}

// Transaction represents a financial transaction
type Transaction struct {
	TxnID  string
	From   string
	To     string
	Amount float64
}

// Ledger manages all accounts (the state machine!)
type Ledger struct {
	mu              sync.RWMutex
	accounts        map[string]*Account
	processedTxns   map[string]bool // For idempotency
}

// NewLedger creates a new ledger
func NewLedger() *Ledger {
	return &Ledger{
		accounts:      make(map[string]*Account),
		processedTxns: make(map[string]bool),
	}
}

// CreateAccount creates a new account with initial balance
func (l *Ledger) CreateAccount(id string, initialBalance float64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if _, exists := l.accounts[id]; exists {
		return fmt.Errorf("account %s already exists", id)
	}
	
	l.accounts[id] = &Account{
		ID:      id,
		Balance: initialBalance,
	}
	
	return nil
}

// GetBalance returns the current balance of an account
func (l *Ledger) GetBalance(id string) (float64, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	account, exists := l.accounts[id]
	if !exists {
		return 0, fmt.Errorf("account %s not found", id)
	}
	
	return account.Balance, nil
}

// Transfer moves money from one account to another (IDEMPOTENT!)
func (l *Ledger) Transfer(txn Transaction) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// IDEMPOTENCY CHECK: Have we already processed this transaction?
	if l.processedTxns[txn.TxnID] {
		// Already processed - this is a duplicate (maybe from replay)
		// Just return success without doing anything
		return nil
	}
	
	// Validate accounts exist
	fromAccount, fromExists := l.accounts[txn.From]
	if !fromExists {
		return fmt.Errorf("source account %s not found", txn.From)
	}
	
	toAccount, toExists := l.accounts[txn.To]
	if !toExists {
		return fmt.Errorf("destination account %s not found", txn.To)
	}
	
	// Validate sufficient funds
	if fromAccount.Balance < txn.Amount {
		return fmt.Errorf("insufficient funds: %s has %.2f, needs %.2f",
			txn.From, fromAccount.Balance, txn.Amount)
	}
	
	// Execute transfer (ACID!)
	fromAccount.Balance -= txn.Amount
	toAccount.Balance += txn.Amount
	
	// Mark as processed (idempotency!)
	l.processedTxns[txn.TxnID] = true
	
	return nil
}

// GetAllAccounts returns all accounts (for debugging)
func (l *Ledger) GetAllAccounts() map[string]float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	result := make(map[string]float64)
	for id, account := range l.accounts {
		result[id] = account.Balance
	}
	return result
}
