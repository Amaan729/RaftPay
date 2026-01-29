package raft

import (
	"encoding/json"
	"log"
)

// Command types that can be in the log
const (
	CommandTypeTransfer       = "TRANSFER"
	CommandTypeCreateAccount = "CREATE_ACCOUNT"
)

// Command is what gets stored in the Raft log
type Command struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// TransferCommand represents a financial transfer
type TransferCommand struct {
	TxnID  string  `json:"txn_id"`
	From   string  `json:"from"`
	To     string  `json:"to"`
	Amount float64 `json:"amount"`
}

// CreateAccountCommand creates a new account
type CreateAccountCommand struct {
	AccountID      string  `json:"account_id"`
	InitialBalance float64 `json:"initial_balance"`
}

// StateMachine interface - what the ledger must implement
type StateMachine interface {
	// Apply a command to the state machine
	Apply(command Command) error
}

// applyCommittedEntries applies entries from lastApplied+1 to commitIndex
func (rn *RaftNode) applyCommittedEntries() {
	// Must be called with rn.mu locked
	
	// Check if there are new entries to apply
	if rn.commitIndex <= rn.lastApplied {
		return // Nothing new to apply
	}
	
	// Apply all committed entries we haven't applied yet
	for i := rn.lastApplied + 1; i <= rn.commitIndex; i++ {
		entry := rn.log[i]
		
		log.Printf("[%s] Applying committed entry %d (term %d) to state machine",
			rn.id, entry.Index, entry.Term)
		
		// Send to state machine via applyCh
		msg := ApplyMsg{
			CommandValid: true,
			Command:      entry.Command,
			CommandIndex: entry.Index,
			CommandTerm:  entry.Term,
		}
		
		// Non-blocking send (in case channel is full)
		select {
		case rn.applyCh <- msg:
			rn.lastApplied = i
			log.Printf("[%s] ✓ Applied entry %d to state machine", rn.id, i)
		default:
			log.Printf("[%s] ⚠ applyCh full, skipping entry %d", rn.id, i)
		}
	}
}
