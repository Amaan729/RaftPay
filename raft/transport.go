package raft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Transport handles network communication between Raft nodes
type Transport struct {
	node      *RaftNode
	serverURL string                // This node's URL (e.g., "http://localhost:8001")
	peerURLs  map[string]string     // Map of peerID → URL
	client    *http.Client
}

// NewTransport creates a new transport layer
func NewTransport(node *RaftNode, serverURL string, peerURLs map[string]string) *Transport {
	return &Transport{
		node:      node,
		serverURL: serverURL,
		peerURLs:  peerURLs,
		client: &http.Client{
			Timeout: node.config.RPCTimeout,
		},
	}
}

// SendRequestVote sends RequestVote RPC to a peer
func (t *Transport) SendRequestVote(peerID string, req *RequestVoteRequest) (*RequestVoteResponse, error) {
	peerURL, ok := t.peerURLs[peerID]
	if !ok {
		return nil, fmt.Errorf("unknown peer: %s", peerID)
	}
	
	// Serialize request to JSON
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	
	// Send HTTP POST
	resp, err := t.client.Post(
		peerURL+"/raft/request_vote",
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Parse response
	var result RequestVoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return &result, nil
}

// SendAppendEntries sends AppendEntries RPC to a peer
func (t *Transport) SendAppendEntries(peerID string, req *AppendEntriesRequest) (*AppendEntriesResponse, error) {
	peerURL, ok := t.peerURLs[peerID]
	if !ok {
		return nil, fmt.Errorf("unknown peer: %s", peerID)
	}
	
	// Serialize request to JSON
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	
	// Send HTTP POST
	resp, err := t.client.Post(
		peerURL+"/raft/append_entries",
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Parse response
	var result AppendEntriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return &result, nil
}
