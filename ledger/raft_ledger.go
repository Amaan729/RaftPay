package ledger

import (
	"encoding/json"
	"log"
	
	"github.com/Amaan729/RaftPay/raft"
)

// RaftLedger wraps a Ledger and applies Raft commands
type RaftLedger struct {
	ledger   *Ledger
	raftNode *raft.RaftNode
	applyCh  chan raft.ApplyMsg
	stopCh   chan struct{}
}

// NewRaftLedger creates a ledger that works with Raft
func NewRaftLedger(raftNode *raft.RaftNode, applyCh chan raft.ApplyMsg) *RaftLedger {
	return &RaftLedger{
		ledger:   NewLedger(),
		raftNode: raftNode,
		applyCh:  applyCh,
		stopCh:   make(chan struct{}),
	}
}

// Start begins listening for committed entries from Raft
func (rl *RaftLedger) Start() {
	log.Println("📚 RaftLedger started - listening for committed entries...")
	go rl.applier()
}

// Stop stops the applier
func (rl *RaftLedger) Stop() {
	close(rl.stopCh)
}

// applier listens for committed entries and applies them to ledger
func (rl *RaftLedger) applier() {
	for {
		select {
		case <-rl.stopCh:
			return
			
		case msg := <-rl.applyCh:
			if !msg.CommandValid {
				continue
			}
			
			// Parse the command
			cmd, ok := msg.Command.(raft.Command)
			if !ok {
				log.Printf("⚠ Invalid command type: %T", msg.Command)
				continue
			}
			
			// Apply based on command type
			switch cmd.Type {
			case raft.CommandTypeTransfer:
				rl.applyTransfer(cmd, msg.CommandIndex)
				
			case raft.CommandTypeCreateAccount:
				rl.applyCreateAccount(cmd, msg.CommandIndex)
				
			default:
				log.Printf("⚠ Unknown command type: %s", cmd.Type)
			}
		}
	}
}

// applyTransfer applies a transfer command to the ledger
func (rl *RaftLedger) applyTransfer(cmd raft.Command, index int) {
	// Deserialize transfer data
	var transfer raft.TransferCommand
	if err := json.Unmarshal(cmd.Data, &transfer); err != nil {
		log.Printf("❌ Failed to unmarshal transfer: %v", err)
		return
	}
	
	// Apply to ledger
	txn := Transaction{
		TxnID:  transfer.TxnID,
		From:   transfer.From,
		To:     transfer.To,
		Amount: transfer.Amount,
	}
	
	if err := rl.ledger.Transfer(txn); err != nil {
		log.Printf("❌ Transfer failed at index %d: %v", index, err)
		return
	}
	
	log.Printf("✅ Applied transfer at index %d: %s→%s $%.2f (txn: %s)",
		index, transfer.From, transfer.To, transfer.Amount, transfer.TxnID)
	
	// Log balances for verification
	fromBalance, _ := rl.ledger.GetBalance(transfer.From)
	toBalance, _ := rl.ledger.GetBalance(transfer.To)
	log.Printf("   Balances: %s=$%.2f, %s=$%.2f", transfer.From, fromBalance, transfer.To, toBalance)
}

// applyCreateAccount creates a new account
func (rl *RaftLedger) applyCreateAccount(cmd raft.Command, index int) {
	// Deserialize create account data
	var create raft.CreateAccountCommand
	if err := json.Unmarshal(cmd.Data, &create); err != nil {
		log.Printf("❌ Failed to unmarshal create account: %v", err)
		return
	}
	
	// Apply to ledger
	if err := rl.ledger.CreateAccount(create.AccountID, create.InitialBalance); err != nil {
		// Ignore "already exists" errors (idempotency)
		log.Printf("⚠ Create account at index %d: %v", index, err)
		return
	}
	
	log.Printf("✅ Applied create account at index %d: %s with $%.2f",
		index, create.AccountID, create.InitialBalance)
}

// GetLedger returns the underlying ledger (for querying balances)
func (rl *RaftLedger) GetLedger() *Ledger {
	return rl.ledger
}
