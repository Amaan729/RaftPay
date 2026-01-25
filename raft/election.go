package raft

import (
	"log"
)

// shouldVoteFor determines if this node should grant a vote to the candidate
func (rn *RaftNode) shouldVoteFor(req *RequestVoteRequest) bool {
	// Rule 1: Reject if candidate's term is less than ours
	if req.Term < rn.currentTerm {
		return false
	}
	
	// Rule 2: Reject if we already voted for someone else this term
	if rn.votedFor != "" && rn.votedFor != req.CandidateID {
		return false
	}
	
	// Rule 3: Reject if candidate's log is not as up-to-date as ours
	if req.LastLogTerm < rn.lastLogTerm() {
		return false
	}
	if req.LastLogTerm == rn.lastLogTerm() && req.LastLogIndex < rn.lastLogIndex() {
		return false
	}
	
	// All checks passed - grant vote!
	return true
}

// startElection begins an election by sending RequestVote RPCs to all peers
func (rn *RaftNode) startElection() {
	// This is called from becomeCandidate() with rn.mu already locked
	
	log.Printf("[%s] Starting election for term %d", rn.id, rn.currentTerm)
	
	// Build RequestVote request
	req := &RequestVoteRequest{
		Term:         rn.currentTerm,
		CandidateID:  rn.id,
		LastLogIndex: rn.lastLogIndex(),
		LastLogTerm:  rn.lastLogTerm(),
	}
	
	// Send RequestVote to all peers in parallel
	for _, peer := range rn.peers {
		go func(peerID string) {
			resp := &RequestVoteResponse{}
			
			// TODO: Send RPC to peer (we'll add RPC client later)
			// For now, simulate the call
			log.Printf("[%s] → RequestVote to %s (term %d)", rn.id, peerID, req.Term)
			
			// Handle response
			rn.mu.Lock()
			rn.handleVoteResponse(resp)
			rn.mu.Unlock()
		}(peer)
	}
}

// handleVoteResponse processes a vote response from a peer
func (rn *RaftNode) handleVoteResponse(resp *RequestVoteResponse) {
	// Must be called with rn.mu locked
	
	// Check if still a candidate (might have won/lost already)
	if rn.state != Candidate {
		return
	}
	
	// If response has higher term, step down
	if resp.Term > rn.currentTerm {
		log.Printf("[%s] Stepping down: saw term %d > currentTerm %d",
			rn.id, resp.Term, rn.currentTerm)
		rn.becomeFollower(resp.Term)
		return
	}
	
	// Count vote if granted
	if resp.VoteGranted && rn.state == Candidate {
		rn.votesReceived++
		log.Printf("[%s] Received vote (%d/%d needed for majority)",
			rn.id, rn.votesReceived, rn.majority)
		
		// Check if won election
		if rn.votesReceived >= rn.majority {
			log.Printf("[%s] WON ELECTION with %d votes!", rn.id, rn.votesReceived)
			rn.becomeLeader()
		}
	}
}
