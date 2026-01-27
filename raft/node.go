package raft

import (
	"log"
	"math/rand"
	"time"
)

// NewRaftNode creates a new Raft node
func NewRaftNode(id string, peers []string, applyCh chan ApplyMsg) *RaftNode {
	totalNodes := len(peers) + 1
	majority := (totalNodes / 2) + 1
	
	rn := &RaftNode{
		id:            id,
		peers:         peers,
		majority:      majority,
		currentTerm:   0,
		votedFor:      "",
		log:           make([]LogEntry, 0),
		commitIndex:   0,
		lastApplied:   0,
		state:         Follower,
		nextIndex:     make(map[string]int),
		matchIndex:    make(map[string]int),
		votesReceived: 0,
		applyCh:       applyCh,
		stopCh:        make(chan struct{}),
		config:        DefaultConfig(),
	}
	
	rn.log = append(rn.log, LogEntry{Term: 0, Index: 0, Command: nil})
	
	log.Printf("[%s] Initialized Raft node | Peers: %v | Majority: %d/%d",
		id, peers, majority, totalNodes)
	
	return rn
}

// SetTransport attaches the transport layer to this node
func (rn *RaftNode) SetTransport(transport *Transport) {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	rn.transport = transport
	log.Printf("[%s] Transport layer attached", rn.id)
}

// Start begins the Raft node's operation
func (rn *RaftNode) Start() {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	
	log.Printf("[%s] Starting Raft node in %s state", rn.id, rn.state)
	rn.becomeFollower(rn.currentTerm)
	go rn.ticker()
}

// Stop gracefully shuts down the Raft node
func (rn *RaftNode) Stop() {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	
	log.Printf("[%s] Stopping Raft node", rn.id)
	close(rn.stopCh)
	
	if rn.electionTimer != nil {
		rn.electionTimer.Stop()
	}
	if rn.heartbeatTimer != nil {
		rn.heartbeatTimer.Stop()
	}
}

// becomeFollower transitions node to follower state
func (rn *RaftNode) becomeFollower(term int) {
	log.Printf("[%s] Becoming FOLLOWER in term %d", rn.id, term)
	
	rn.state = Follower
	rn.currentTerm = term
	rn.votedFor = ""
	rn.votesReceived = 0
	
	if rn.heartbeatTimer != nil {
		rn.heartbeatTimer.Stop()
	}
	
	rn.resetElectionTimer()
}

// becomeCandidate transitions node to candidate state and starts election
func (rn *RaftNode) becomeCandidate() {
	log.Printf("[%s] Becoming CANDIDATE in term %d", rn.id, rn.currentTerm+1)
	
	rn.state = Candidate
	rn.currentTerm++
	rn.votedFor = rn.id
	rn.votesReceived = 1
	
	rn.resetElectionTimer()
	
	// Start election with the logic YOU implemented!
	rn.startElection()
}

// becomeLeader transitions node to leader state
func (rn *RaftNode) becomeLeader() {
	log.Printf("[%s] Becoming LEADER in term %d", rn.id, rn.currentTerm)
	
	rn.state = Leader
	
	if rn.electionTimer != nil {
		rn.electionTimer.Stop()
	}
	
	lastLogIndex := len(rn.log) - 1
	for _, peer := range rn.peers {
		rn.nextIndex[peer] = lastLogIndex + 1
		rn.matchIndex[peer] = 0
	}
	
	// Send initial heartbeats with replication
	rn.sendHeartbeats()
	rn.resetHeartbeatTimer()
}

// resetElectionTimer resets the election timeout with randomization
func (rn *RaftNode) resetElectionTimer() {
	timeout := rn.config.ElectionTimeoutMin +
		time.Duration(rand.Int63n(int64(rn.config.ElectionTimeoutMax-rn.config.ElectionTimeoutMin)))
	
	if rn.electionTimer == nil {
		rn.electionTimer = time.AfterFunc(timeout, func() {
			rn.mu.Lock()
			defer rn.mu.Unlock()
			
			if rn.state != Leader {
				rn.becomeCandidate()
			}
		})
	} else {
		rn.electionTimer.Reset(timeout)
	}
}

// resetHeartbeatTimer resets the heartbeat timer for leaders
func (rn *RaftNode) resetHeartbeatTimer() {
	if rn.heartbeatTimer == nil {
		rn.heartbeatTimer = time.AfterFunc(rn.config.HeartbeatInterval, func() {
			rn.mu.Lock()
			defer rn.mu.Unlock()
			
			if rn.state == Leader {
				rn.sendHeartbeats()
				rn.resetHeartbeatTimer()
			}
		})
	} else {
		rn.heartbeatTimer.Reset(rn.config.HeartbeatInterval)
	}
}

// ticker is a background goroutine
func (rn *RaftNode) ticker() {
	for {
		select {
		case <-rn.stopCh:
			return
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// GetState returns current term and whether this node is the leader
func (rn *RaftNode) GetState() (term int, isLeader bool) {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	return rn.currentTerm, rn.state == Leader
}

// sendHeartbeats sends AppendEntries to all peers (with YOUR replication logic!)
func (rn *RaftNode) sendHeartbeats() {
	if rn.state != Leader {
		return
	}
	
	log.Printf("[%s] Sending heartbeats/replication (term %d)", rn.id, rn.currentTerm)
	
	// Use the replication logic YOU implemented!
	rn.replicateLog()
}
