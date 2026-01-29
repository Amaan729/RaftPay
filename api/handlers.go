package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	
	"github.com/Amaan729/RaftPay/ledger"
	"github.com/Amaan729/RaftPay/raft"
)

// APIServer wraps the Raft node and ledger with HTTP endpoints
type APIServer struct {
	node       *raft.RaftNode
	ledger     *ledger.RaftLedger
	nodeID     string
	apiPort    string // Port for this API server (e.g., ":9001")
}

// NewAPIServer creates a new REST API server
func NewAPIServer(node *raft.RaftNode, ledger *ledger.RaftLedger, nodeID string, apiPort string) *APIServer {
	return &APIServer{
		node:    node,
		ledger:  ledger,
		nodeID:  nodeID,
		apiPort: apiPort,
	}
}

// HandleCreateAccount handles POST /account
func (s *APIServer) HandleCreateAccount(w http.ResponseWriter, r *http.Request) {
	// Only accept POST
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	
	// Parse request
	var req CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}
	
	// Validate
	if req.AccountID == "" {
		s.respondError(w, http.StatusBadRequest, "account_id is required")
		return
	}
	if req.InitialBalance < 0 {
		s.respondError(w, http.StatusBadRequest, "initial_balance must be >= 0")
		return
	}
	
	// Check if we're the leader
	term, isLeader := s.node.GetState()
	if !isLeader {
		s.respondNotLeader(w)
		return
	}
	
	// Propose to Raft
	index, term, isLeader, err := s.node.ProposeCreateAccount(req.AccountID, req.InitialBalance)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to propose: %v", err))
		return
	}
	
	if !isLeader {
		s.respondNotLeader(w)
		return
	}
	
	// Success!
	resp := CreateAccountResponse{
		Status:  "accepted",
		Message: fmt.Sprintf("Account creation proposed at index %d", index),
		Index:   index,
		Term:    term,
	}
	
	s.respondJSON(w, http.StatusOK, resp)
	log.Printf("[API:%s] ✅ Account creation accepted: %s with $%.2f", s.nodeID, req.AccountID, req.InitialBalance)
}

// HandleTransfer handles POST /transfer
func (s *APIServer) HandleTransfer(w http.ResponseWriter, r *http.Request) {
	// Only accept POST
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	
	// Parse request
	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}
	
	// Validate
	if req.From == "" || req.To == "" {
		s.respondError(w, http.StatusBadRequest, "from and to are required")
		return
	}
	if req.Amount <= 0 {
		s.respondError(w, http.StatusBadRequest, "amount must be > 0")
		return
	}
	if req.TxnID == "" {
		s.respondError(w, http.StatusBadRequest, "txn_id is required for idempotency")
		return
	}
	
	// Check if we're the leader
	term, isLeader := s.node.GetState()
	if !isLeader {
		s.respondNotLeader(w)
		return
	}
	
	// Propose to Raft
	index, term, isLeader, err := s.node.ProposeTransfer(req.From, req.To, req.Amount, req.TxnID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to propose: %v", err))
		return
	}
	
	if !isLeader {
		s.respondNotLeader(w)
		return
	}
	
	// Success!
	resp := TransferResponse{
		Status:  "accepted",
		Message: fmt.Sprintf("Transfer proposed at index %d", index),
		Index:   index,
		Term:    term,
	}
	
	s.respondJSON(w, http.StatusOK, resp)
	log.Printf("[API:%s] ✅ Transfer accepted: %s→%s $%.2f (txn: %s)", 
		s.nodeID, req.From, req.To, req.Amount, req.TxnID)
}

// HandleGetBalance handles GET /balance/:id
func (s *APIServer) HandleGetBalance(w http.ResponseWriter, r *http.Request) {
	// Only accept GET
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	
	// Extract account ID from URL path
	// URL format: /balance/alice
	accountID := r.URL.Path[len("/balance/"):]
	if accountID == "" {
		s.respondError(w, http.StatusBadRequest, "account_id is required")
		return
	}
	
	// Query local ledger (read-only, no consensus needed)
	balance, err := s.ledger.GetLedger().GetBalance(accountID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, fmt.Sprintf("Account not found: %v", err))
		return
	}
	
	// Success!
	resp := BalanceResponse{
		AccountID: accountID,
		Balance:   balance,
		NodeID:    s.nodeID,
	}
	
	s.respondJSON(w, http.StatusOK, resp)
}

// HandleGetStatus handles GET /status
func (s *APIServer) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	// Only accept GET
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	
	// Get node state
	term, isLeader := s.node.GetState()
	state := "FOLLOWER"
	if isLeader {
		state = "LEADER"
	}
	
	// Get additional stats (we'll add a method to RaftNode for this)
	commitIndex, lastApplied, logLength := s.node.GetStats()
	
	resp := StatusResponse{
		NodeID:      s.nodeID,
		State:       state,
		Term:        term,
		CommitIndex: commitIndex,
		LastApplied: lastApplied,
		LogLength:   logLength,
		Peers:       len(s.node.GetPeers()),
	}
	
	s.respondJSON(w, http.StatusOK, resp)
}

// Helper: Respond with JSON
func (s *APIServer) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Helper: Respond with error
func (s *APIServer) respondError(w http.ResponseWriter, status int, message string) {
	resp := ErrorResponse{
		Error: message,
	}
	s.respondJSON(w, status, resp)
}

// Helper: Respond with "not leader" error
func (s *APIServer) respondNotLeader(w http.ResponseWriter) {
	// In a real system, we'd track the leader and provide a hint
	// For now, just tell them we're not the leader
	resp := ErrorResponse{
		Error:      "not leader",
		LeaderHint: "Try another node (leader election in progress or this is a follower)",
	}
	s.respondJSON(w, http.StatusServiceUnavailable, resp)
}

// StartServer starts the HTTP server
func (s *APIServer) StartServer() error {
	mux := http.NewServeMux()
	
	// Register routes
	mux.HandleFunc("/account", s.HandleCreateAccount)
	mux.HandleFunc("/transfer", s.HandleTransfer)
	mux.HandleFunc("/balance/", s.HandleGetBalance)
	mux.HandleFunc("/status", s.HandleGetStatus)
	
	log.Printf("[API:%s] Starting API server on %s", s.nodeID, s.apiPort)
	return http.ListenAndServe(s.apiPort, mux)
}
