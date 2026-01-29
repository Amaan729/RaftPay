package raft

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

// PersistentState represents the durable state that must survive crashes
type PersistentState struct {
	CurrentTerm int    `json:"current_term"`
	VotedFor    string `json:"voted_for"`
}

// Persister handles saving and loading Raft state to/from disk
type Persister struct {
	dataDir   string // Directory for this node's data
	stateFile string // Path to state.json
	logFile   string // Path to log.jsonl
}

// NewPersister creates a new persister for a given node
func NewPersister(nodeID string) *Persister {
	// Create data directory structure: data/nodeID/
	dataDir := filepath.Join("data", nodeID)
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}
	
	return &Persister{
		dataDir:   dataDir,
		stateFile: filepath.Join(dataDir, "state.json"),
		logFile:   filepath.Join(dataDir, "log.jsonl"),
	}
}

// SaveState atomically saves currentTerm and votedFor to disk
func (p *Persister) SaveState(term int, votedFor string) error {
	state := PersistentState{
		CurrentTerm: term,
		VotedFor:    votedFor,
	}
	
	// Marshal to JSON
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %v", err)
	}
	
	// Write to temp file first (atomic write pattern)
	tempFile := p.stateFile + ".tmp"
	if err := ioutil.WriteFile(tempFile, data, 0644); err != nil {
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
	// Check if state file exists
	if _, err := os.Stat(p.stateFile); os.IsNotExist(err) {
		return 0, "", false, nil // No state file, fresh start
	}
	
	// Read file
	data, err := ioutil.ReadFile(p.stateFile)
	if err != nil {
		return 0, "", false, fmt.Errorf("failed to read state file: %v", err)
	}
	
	// Unmarshal
	var state PersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return 0, "", false, fmt.Errorf("failed to unmarshal state: %v", err)
	}
	
	return state.CurrentTerm, state.VotedFor, true, nil
}

// AppendLogEntry appends a single log entry to the log file
func (p *Persister) AppendLogEntry(entry LogEntry) error {
	// Open file in append mode (create if doesn't exist)
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
	
	// Write as single line (JSONL format)
	if _, err := f.WriteString(string(data) + "\n"); err != nil {
		return fmt.Errorf("failed to write log entry: %v", err)
	}
	
	return nil
}

// LoadLog loads all log entries from disk
// Returns the log and any error
func (p *Persister) LoadLog() ([]LogEntry, error) {
	// Check if log file exists
	if _, err := os.Stat(p.logFile); os.IsNotExist(err) {
		// No log file, return empty log with dummy entry at index 0
		return []LogEntry{{Term: 0, Index: 0, Command: nil}}, nil
	}
	
	// Open file for reading
	f, err := os.Open(p.logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}
	defer f.Close()
	
	// Start with dummy entry at index 0
	log := []LogEntry{{Term: 0, Index: 0, Command: nil}}
	
	// Read line by line (JSONL format)
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal log entry at line %d: %v", lineNum, err)
		}
		
		log = append(log, entry)
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %v", err)
	}
	
	return log, nil
}

// TruncateLog truncates the log file to only contain entries up to lastIndex
// This is used when a follower needs to delete conflicting entries
func (p *Persister) TruncateLog(entries []LogEntry) error {
	// Remove old log file
	if err := os.Remove(p.logFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove old log file: %v", err)
	}
	
	// Write all entries (except dummy at index 0)
	for i := 1; i < len(entries); i++ {
		if err := p.AppendLogEntry(entries[i]); err != nil {
			return err
		}
	}
	
	return nil
}

// GetDataDir returns the data directory for this node
func (p *Persister) GetDataDir() string {
	return p.dataDir
}

// DeleteAll removes all persisted state (for testing)
func (p *Persister) DeleteAll() error {
	return os.RemoveAll(p.dataDir)
}
