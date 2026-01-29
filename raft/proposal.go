package raft

import (
	"encoding/json"
	"fmt"
	"log"
)

// Propose submits a command to the Raft cluster
// Returns: (index, term, isLeader)
func (rn *RaftNode) Propose(command Command) (int, int, bool) {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	
	// Only leader can accept proposals
	if rn.state != Leader {
		return -1, rn.currentTerm, false
	}
	
	// Add to our log
	newIndex := len(rn.log)
	entry := LogEntry{
		Term:    rn.currentTerm,
		Index:   newIndex,
		Command: command,
	}
	rn.log = append(rn.log, entry)
	
	log.Printf("[%s] 📝 Proposed new entry at index %d (term %d)",
		rn.id, newIndex, rn.currentTerm)
	
	// Immediately replicate to followers
	rn.replicateLog()
	
	return newIndex, rn.currentTerm, true
}

// ProposeTransfer is a convenience method for submitting transfers
func (rn *RaftNode) ProposeTransfer(from, to string, amount float64, txnID string) (int, int, bool, error) {
	// Build transfer command
	transferCmd := TransferCommand{
		TxnID:  txnID,
		From:   from,
		To:     to,
		Amount: amount,
	}
	
	// Serialize to JSON
	data, err := json.Marshal(transferCmd)
	if err != nil {
		return -1, -1, false, fmt.Errorf("failed to serialize transfer: %v", err)
	}
	
	// Create command
	cmd := Command{
		Type: CommandTypeTransfer,
		Data: json.RawMessage(data),
	}
	
	// Propose to Raft
	index, term, isLeader := rn.Propose(cmd)
	if !isLeader {
		return index, term, false, fmt.Errorf("not leader")
	}
	
	log.Printf("[%s] 💰 Proposed transfer: %s→%s $%.2f (txn: %s) at index %d",
		rn.id, from, to, amount, txnID, index)
	
	return index, term, true, nil
}

// ProposeCreateAccount is a convenience method for creating accounts
func (rn *RaftNode) ProposeCreateAccount(accountID string, initialBalance float64) (int, int, bool, error) {
	// Build create account command
	createCmd := CreateAccountCommand{
		AccountID:      accountID,
		InitialBalance: initialBalance,
	}
	
	// Serialize to JSON
	data, err := json.Marshal(createCmd)
	if err != nil {
		return -1, -1, false, fmt.Errorf("failed to serialize create account: %v", err)
	}
	
	// Create command
	cmd := Command{
		Type: CommandTypeCreateAccount,
		Data: json.RawMessage(data),
	}
	
	// Propose to Raft
	index, term, isLeader := rn.Propose(cmd)
	if !isLeader {
		return index, term, false, fmt.Errorf("not leader")
	}
	
	log.Printf("[%s] 🏦 Proposed create account: %s with $%.2f at index %d",
		rn.id, accountID, initialBalance, index)
	
	return index, term, true, nil
}
