package raft

import (
	"encoding/json"
	"log"
	"net/http"
)

// HTTPServer handles incoming Raft RPC requests
type HTTPServer struct {
	node *RaftNode
}

// NewHTTPServer creates a new HTTP server for Raft RPCs
func NewHTTPServer(node *RaftNode) *HTTPServer {
	return &HTTPServer{node: node}
}

// ServeHTTP handles Raft RPC endpoints
func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/raft/request_vote":
		s.handleRequestVote(w, r)
	case "/raft/append_entries":
		s.handleAppendEntries(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleRequestVote handles RequestVote RPC
func (s *HTTPServer) handleRequestVote(w http.ResponseWriter, r *http.Request) {
	var req RequestVoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	var resp RequestVoteResponse
	if err := s.node.RequestVote(&req, &resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleAppendEntries handles AppendEntries RPC
func (s *HTTPServer) handleAppendEntries(w http.ResponseWriter, r *http.Request) {
	var req AppendEntriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	var resp AppendEntriesResponse
	if err := s.node.AppendEntries(&req, &resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// StartServer starts the HTTP server on the given address
func (s *HTTPServer) StartServer(addr string) error {
	log.Printf("Starting Raft HTTP server on %s", addr)
	return http.ListenAndServe(addr, s)
}
