// Package raft implements the Raft consensus algorithm.
// Based on the Raft paper: https://raft.github.io/raft.pdf
package raft

import (
    "sync"
    "time"
)

// NodeState represents the three states a Raft node can be in
type NodeState int

const (
    Follower  NodeState = iota // Default state, receives AppendEntries and votes
    Candidate                   // Actively requesting votes
    Leader                      // Processes client requests, replicates log
)

func (s NodeState) String() string {
    switch s {
    case Follower:
        return "FOLLOWER"
    case Candidate:
        return "CANDIDATE"
    case Leader:
        return "LEADER"
    default:
        return "UNKNOWN"
    }
}

// LogEntry represents a single command in the replicated log
type LogEntry struct {
    Term    int         // Term when entry was created by leader
    Index   int         // Position in log (1-indexed)
    Command interface{} // The actual command (e.g., financial transaction)
}

// RaftNode is the core Raft consensus module
type RaftNode struct {
    mu sync.Mutex // Protects all fields below

    // --- Persistent State (survives crashes) ---
    currentTerm int        // Latest term this server has seen
    votedFor    string     // CandidateID that received vote in current term
    log         []LogEntry // Log entries; first index is 1

    // --- Volatile State (all servers) ---
    commitIndex int       // Index of highest log entry known to be committed
    lastApplied int       // Index of highest log entry applied to state machine
    state       NodeState // Current state (Follower/Candidate/Leader)

    // --- Volatile State (leaders only) ---
    nextIndex  map[string]int // For each server, index of next log entry to send
    matchIndex map[string]int // For each server, index of highest log entry known to be replicated

    // --- Cluster Configuration ---
    id       string   // This node's unique ID
    peers    []string // Other nodes in cluster
    majority int      // Votes needed to win election

    // --- Election State ---
    votesReceived  int          // Votes received in current election
    electionTimer  *time.Timer  // Triggers election when timeout expires
    heartbeatTimer *time.Timer  // Leaders send heartbeats on this timer

    // --- Communication Channels ---
    applyCh chan ApplyMsg // Send committed entries here for state machine

    // --- Lifecycle ---
    stopCh chan struct{} // Close to shutdown all goroutines

    // --- Configuration ---
    config *Config
}

// ApplyMsg is sent when a log entry is committed and ready to apply
type ApplyMsg struct {
    CommandValid bool
    Command      interface{}
    CommandIndex int
    CommandTerm  int
}

// RequestVoteRequest is sent by candidates to gather votes
type RequestVoteRequest struct {
    Term         int
    CandidateID  string
    LastLogIndex int
    LastLogTerm  int
}

// RequestVoteResponse is the reply to RequestVote
type RequestVoteResponse struct {
    Term        int
    VoteGranted bool
}

// AppendEntriesRequest is sent by leader to replicate log entries
type AppendEntriesRequest struct {
    Term         int
    LeaderID     string
    PrevLogIndex int
    PrevLogTerm  int
    Entries      []LogEntry
    LeaderCommit int
}

// AppendEntriesResponse is the reply to AppendEntries
type AppendEntriesResponse struct {
    Term          int
    Success       bool
    ConflictTerm  int
    ConflictIndex int
}
