package raft

import "log"

// RequestVote RPC handler
// Called by candidates to gather votes
func (rn *RaftNode) RequestVote(req *RequestVoteRequest, resp *RequestVoteResponse) error {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	
	log.Printf("[%s] Received RequestVote from %s (term %d, lastLog: %d/%d)",
		rn.id, req.CandidateID, req.Term, req.LastLogTerm, req.LastLogIndex)
	
	// Initialize response with current term
	resp.Term = rn.currentTerm
	resp.VoteGranted = false
	
	// TODO: Implement voting logic (election.go)
	// For now, just update term if candidate has higher term
	if req.Term > rn.currentTerm {
		rn.becomeFollower(req.Term)
		resp.Term = rn.currentTerm
	}
	
	return nil
}

// AppendEntries RPC handler
// Called by leader to replicate log entries (also used as heartbeat)
func (rn *RaftNode) AppendEntries(req *AppendEntriesRequest, resp *AppendEntriesResponse) error {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	
	log.Printf("[%s] Received AppendEntries from %s (term %d, prevLog: %d/%d, entries: %d)",
		rn.id, req.LeaderID, req.Term, req.PrevLogTerm, req.PrevLogIndex, len(req.Entries))
	
	// Initialize response
	resp.Term = rn.currentTerm
	resp.Success = false
	
	// Rule 1: Reply false if term < currentTerm
	if req.Term < rn.currentTerm {
		log.Printf("[%s] Rejecting AppendEntries: term %d < currentTerm %d",
			rn.id, req.Term, rn.currentTerm)
		return nil
	}
	
	// If RPC request or response contains term > currentTerm:
	// set currentTerm = T, convert to follower
	if req.Term > rn.currentTerm {
		rn.becomeFollower(req.Term)
		resp.Term = rn.currentTerm
	}
	
	// Valid leader for current term - reset election timer
	rn.resetElectionTimer()
	
	// Ensure we're a follower (could be candidate with same term)
	if rn.state != Follower {
		rn.becomeFollower(req.Term)
	}
	
	// TODO: Implement log consistency check and replication (replication.go)
	
	return nil
}
