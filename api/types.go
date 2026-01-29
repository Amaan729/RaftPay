package api

// CreateAccountRequest is the request body for POST /account
type CreateAccountRequest struct {
	AccountID      string  `json:"account_id"`
	InitialBalance float64 `json:"initial_balance"`
}

// CreateAccountResponse is the response for POST /account
type CreateAccountResponse struct {
	Status  string `json:"status"`  // "accepted" or "rejected"
	Message string `json:"message"` // Human-readable message
	Index   int    `json:"index"`   // Log index where command was added
	Term    int    `json:"term"`    // Current term
}

// TransferRequest is the request body for POST /transfer
type TransferRequest struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Amount float64 `json:"amount"`
	TxnID  string  `json:"txn_id"` // Client-provided transaction ID for idempotency
}

// TransferResponse is the response for POST /transfer
type TransferResponse struct {
	Status  string `json:"status"`  // "accepted" or "rejected"
	Message string `json:"message"` // Human-readable message
	Index   int    `json:"index"`   // Log index where command was added
	Term    int    `json:"term"`    // Current term
}

// BalanceResponse is the response for GET /balance/:id
type BalanceResponse struct {
	AccountID string  `json:"account_id"`
	Balance   float64 `json:"balance"`
	NodeID    string  `json:"node_id"` // Which node served this request
}

// StatusResponse is the response for GET /status
type StatusResponse struct {
	NodeID      string `json:"node_id"`       // This node's ID
	State       string `json:"state"`         // "LEADER", "FOLLOWER", or "CANDIDATE"
	Term        int    `json:"term"`          // Current term
	CommitIndex int    `json:"commit_index"`  // Highest committed log index
	LastApplied int    `json:"last_applied"`  // Highest applied log index
	LogLength   int    `json:"log_length"`    // Total log entries
	Peers       int    `json:"peers"`         // Number of peers
}

// ErrorResponse is returned when an error occurs
type ErrorResponse struct {
	Error      string `json:"error"`                 // Error message
	LeaderHint string `json:"leader_hint,omitempty"` // If follower, hint where leader might be
}
