package raft

import "log"

// checkLogConsistency verifies that follower's log matches leader's at PrevLogIndex
func (rn *RaftNode) checkLogConsistency(req *AppendEntriesRequest) bool {
	// Special case: no previous entry to check (beginning of log)
	if req.PrevLogIndex == 0 {
		return true
	}
	
	// Get the entry at PrevLogIndex
	entry := rn.getLogEntry(req.PrevLogIndex)
	if entry == nil {
		// We don't have an entry at that index
		return false
	}
	
	// Check if the term matches
	return entry.Term == req.PrevLogTerm
}

// appendNewEntries adds entries from leader, handling conflicts
func (rn *RaftNode) appendNewEntries(req *AppendEntriesRequest) {
	// Start inserting at PrevLogIndex + 1
	insertIndex := req.PrevLogIndex + 1
	
	for i, entry := range req.Entries {
		currentIndex := insertIndex + i
		
		// If we already have an entry at this index
		if currentIndex < len(rn.log) {
			// If terms match, we already have this entry - skip it
			if rn.log[currentIndex].Term == entry.Term {
				continue
			}
			
			// Terms don't match - there's a conflict!
			// Delete this entry and everything after it
			log.Printf("[%s] Conflict at index %d: our term %d vs leader's term %d - truncating",
				rn.id, currentIndex, rn.log[currentIndex].Term, entry.Term)
			rn.log = rn.log[:currentIndex]
		}
		
		// Append this entry (and set its index)
		entry.Index = currentIndex
		rn.log = append(rn.log, entry)
		log.Printf("[%s] Appended entry at index %d (term %d)",
			rn.id, currentIndex, entry.Term)
	}
}

// updateCommitIndex updates follower's commit index based on leader's
func (rn *RaftNode) updateCommitIndex(req *AppendEntriesRequest) {
	// Only update if leader's commit is higher than ours
	if req.LeaderCommit > rn.commitIndex {
		// But don't commit beyond what we actually have!
		newCommitIndex := min(req.LeaderCommit, rn.lastLogIndex())
		
		log.Printf("[%s] Updating commitIndex: %d → %d (leader: %d, lastIndex: %d)",
			rn.id, rn.commitIndex, newCommitIndex, req.LeaderCommit, rn.lastLogIndex())
		
		rn.commitIndex = newCommitIndex
		
		// TODO: Apply committed entries to state machine (next batch!)
	}
}

// sendAppendEntriesToPeer sends AppendEntries RPC to a single peer
func (rn *RaftNode) sendAppendEntriesToPeer(peerID string) {
	// Get the next index we need to send to this peer
	nextIdx := rn.nextIndex[peerID]
	
	// Get the previous log entry (for consistency check)
	prevLogIndex := nextIdx - 1
	prevLogTerm := 0
	if prevLogIndex > 0 {
		prevLogTerm = rn.log[prevLogIndex].Term
	}
	
	// Get entries to send (from nextIdx to end of log)
	var entries []LogEntry
	if nextIdx <= rn.lastLogIndex() {
		entries = make([]LogEntry, len(rn.log[nextIdx:]))
		copy(entries, rn.log[nextIdx:])
	}
	
	// Build request
	req := &AppendEntriesRequest{
		Term:         rn.currentTerm,
		LeaderID:     rn.id,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      entries,
		LeaderCommit: rn.commitIndex,
	}
	
	// Send RPC in background
	go func() {
		resp := &AppendEntriesResponse{}
		
		// TODO: Send actual RPC (network layer coming soon)
		log.Printf("[%s] → AppendEntries to %s (prevLog: %d/%d, entries: %d, commit: %d)",
			rn.id, peerID, prevLogIndex, prevLogTerm, len(entries), rn.commitIndex)
		
		// Handle response
		rn.mu.Lock()
		rn.handleAppendEntriesResponse(peerID, req, resp)
		rn.mu.Unlock()
	}()
}

// handleAppendEntriesResponse processes AppendEntries response from a follower
func (rn *RaftNode) handleAppendEntriesResponse(peerID string, req *AppendEntriesRequest, resp *AppendEntriesResponse) {
	// Ignore if we're no longer leader
	if rn.state != Leader {
		return
	}
	
	// If response has higher term, step down
	if resp.Term > rn.currentTerm {
		log.Printf("[%s] Stepping down: saw term %d in AppendEntries response",
			rn.id, resp.Term)
		rn.becomeFollower(resp.Term)
		return
	}
	
	if resp.Success {
		// Success! Update our tracking
		newMatchIndex := req.PrevLogIndex + len(req.Entries)
		rn.matchIndex[peerID] = newMatchIndex
		rn.nextIndex[peerID] = newMatchIndex + 1
		
		log.Printf("[%s] ✓ %s acknowledged up to index %d",
			rn.id, peerID, newMatchIndex)
		
		// Check if we can advance commitIndex
		rn.advanceCommitIndex()
	} else {
		// Failure - follower's log is inconsistent
		// Decrement nextIndex and retry
		rn.nextIndex[peerID]--
		if rn.nextIndex[peerID] < 1 {
			rn.nextIndex[peerID] = 1
		}
		
		log.Printf("[%s] ✗ %s rejected - decrementing nextIndex to %d",
			rn.id, peerID, rn.nextIndex[peerID])
		
		// Retry immediately
		rn.sendAppendEntriesToPeer(peerID)
	}
}

// advanceCommitIndex checks if we can commit more entries (as leader)
func (rn *RaftNode) advanceCommitIndex() {
	// Only leader calls this
	if rn.state != Leader {
		return
	}
	
	// Try to find the highest index replicated on majority
	for n := rn.lastLogIndex(); n > rn.commitIndex; n-- {
		// Can only commit entries from current term (safety rule!)
		if rn.log[n].Term != rn.currentTerm {
			continue
		}
		
		// Count how many servers have this entry
		replicaCount := 1 // Count self
		for _, peer := range rn.peers {
			if rn.matchIndex[peer] >= n {
				replicaCount++
			}
		}
		
		// Do we have majority?
		if replicaCount >= rn.majority {
			log.Printf("[%s] Entry %d replicated on %d/%d servers - committing!",
				rn.id, n, replicaCount, len(rn.peers)+1)
			
			rn.commitIndex = n
			
			// TODO: Apply committed entries to state machine
			
			break // Found the highest committable index
		}
	}
}

// replicateLog sends AppendEntries to all followers (called by leader)
func (rn *RaftNode) replicateLog() {
	if rn.state != Leader {
		return
	}
	
	// Send to all peers
	for _, peer := range rn.peers {
		rn.sendAppendEntriesToPeer(peer)
	}
}
