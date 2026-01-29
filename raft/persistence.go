package raft

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// PersistentState represents the state that must survive crashes
type PersistentState struct {
	CurrentTerm int    `json:"current_term"`
	VotedFor    string `json:"voted_for"`
}

// Persister handles saving/loading Raft state to/from disk
type Persister struct {
	mu          sync.Mutex
	dataDir     string // Directory for this node's data
	stateFile   string // Path to state.json
	logFile     string // Path to log.jsonl
}

// NewPersister creates a new persister for a node
func NewPersister(nodeID string) *Persister {
	dataDir := filepath.Join("data", nodeID)
	
	// Create data directory if it doesn't exist
	os.MkdirAll(dataDir, 0755)
	
	return &Persister{
		dataDir:   dataDir,
		stateFile: filepath.Join(dataDir, "state.json"),
		logFile:   filepath.Join(dataDir, "log.jsonl"),
	}
}

// SaveState atomically saves currentTerm and votedFor to disk
func (p *Persister) SaveState(term int, votedFor string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	state := PersistentState{
		CurrentTerm: term,
		VotedFor:    votedFor,
	}
	
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %v", err)
	}
	
	// Write to temp file first, then atomically rename
	tempFile := p.stateFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp state file: %v", err)
	}
	
	// Atomic rename (POSIX guarantees atomicity)
	if err := os.Rename(tempFile, p.stateFile); err != nil {
		return fmt.Errorf("failed to rename state file: %v", err)
	}
	
	return nil
}

// LoadState loads currentTerm and votedFor from disk
// Returns (term, votedFor, exists, error)
func (p *Persister) LoadState() (int, string, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	data, err := os.ReadFile(p.stateFile)
	if os.IsNotExist(err) {
		// No saved state - this is a fresh start
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, fmt.Errorf("failed to read state file: %v", err)
	}
	
	var state PersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return 0, "", false, fmt.Errorf("failed to unmarshal state: %v", err)
	}
	
	return state.CurrentTerm, state.VotedFor, true, nil
}

// AppendLogEntry appends a log entry to disk
func (p *Persister) AppendLogEntry(entry LogEntry) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Open log file in append mode
	f, err := os.OpenFile(p.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer f.Close()
	
	// Marshal entry to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %v", err)
	}
	
	// Write entry as a line (JSONL format)
	if _, err := f.WriteString(string(data) + "\n"); err != nil {
		return fmt.Errorf("failed to write log entry: %v", err)
	}
	
	// Sync to disk for durability
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync log file: %v", err)
	}
	
	return nil
}

// LoadLog loads all log entries from disk
func (p *Persister) LoadLog() ([]LogEntry, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	f, err := os.Open(p.logFile)
	if os.IsNotExist(err) {
		// No log file - return empty log with dummy entry at index 0
		return []LogEntry{{Term: 0, Index: 0, Command: nil}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}
	defer f.Close()
	
	// Start with dummy entry at index 0
	log := []LogEntry{{Term: 0, Index: 0, Command: nil}}
	
	// Read log entries line by line
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal log entry: %v", err)
		}
		log = append(log, entry)
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read log file: %v", err)
	}
	
	return log, nil
}

// TruncateLog truncates the log file to only contain entries up to lastIndex
// Used when a follower needs to delete conflicting entries
func (p *Persister) TruncateLog(entries []LogEntry) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Write to temp file
	tempFile := p.logFile + ".tmp"
	f, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp log file: %v", err)
	}
	
	// Write all entries except dummy entry at index 0
	for i := 1; i < len(entries); i++ {
		data, err := json.Marshal(entries[i])
		if err != nil {
			f.Close()
			return fmt.Errorf("failed to marshal log entry: %v", err)
		}
		
		if _, err := f.WriteString(string(data) + "\n"); err != nil {
			f.Close()
			return fmt.Errorf("failed to write log entry: %v", err)
		}
	}
	
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("failed to sync log file: %v", err)
	}
	f.Close()
	
	// Atomic rename
	if err := os.Rename(tempFile, p.logFile); err != nil {
		return fmt.Errorf("failed to rename log file: %v", err)
	}
	
	return nil
}

// CreateSnapshot creates a snapshot of the log up to lastIncludedIndex
// This is used for log compaction (optional optimization)
func (p *Persister) CreateSnapshot(lastIncludedIndex int, lastIncludedTerm int, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	snapshotFile := filepath.Join(p.dataDir, "snapshot.json")
	
	snapshot := struct {
		LastIncludedIndex int    `json:"last_included_index"`
		LastIncludedTerm  int    `json:"last_included_term"`
		Data              []byte `json:"data"`
	}{
		LastIncludedIndex: lastIncludedIndex,
		LastIncludedTerm:  lastIncludedTerm,
		Data:              data,
	}
	
	snapshotData, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %v", err)
	}
	
	// Write to temp file, then atomically rename
	tempFile := snapshotFile + ".tmp"
	if err := os.WriteFile(tempFile, snapshotData, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot: %v", err)
	}
	
	if err := os.Rename(tempFile, snapshotFile); err != nil {
		return fmt.Errorf("failed to rename snapshot: %v", err)
	}
	
	return nil
}

// LoadSnapshot loads the latest snapshot (returns nil if no snapshot exists)
func (p *Persister) LoadSnapshot() (lastIncludedIndex int, lastIncludedTerm int, data []byte, exists bool, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	snapshotFile := filepath.Join(p.dataDir, "snapshot.json")
	
	snapshotData, err := os.ReadFile(snapshotFile)
	if os.IsNotExist(err) {
		return 0, 0, nil, false, nil
	}
	if err != nil {
		return 0, 0, nil, false, fmt.Errorf("failed to read snapshot: %v", err)
	}
	
	var snapshot struct {
		LastIncludedIndex int    `json:"last_included_index"`
		LastIncludedTerm  int    `json:"last_included_term"`
		Data              []byte `json:"data"`
	}
	
	if err := json.Unmarshal(snapshotData, &snapshot); err != nil {
		return 0, 0, nil, false, fmt.Errorf("failed to unmarshal snapshot: %v", err)
	}
	
	return snapshot.LastIncludedIndex, snapshot.LastIncludedTerm, snapshot.Data, true, nil
}

// ClearAll removes all persistent data (useful for testing)
func (p *Persister) ClearAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	return os.RemoveAll(p.dataDir)
}

// GetDataDir returns the data directory path
func (p *Persister) GetDataDir() string {
	return p.dataDir
}
