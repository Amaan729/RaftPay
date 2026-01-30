package raft

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// PersistentState represents the durable state that must survive crashes
type PersistentState struct {
	CurrentTerm int    `json:"current_term"`
	VotedFor    string `json:"voted_for"`
}

// Persister handles saving and loading Raft state to/from disk
type Persister struct {
	mu        sync.Mutex
	dataDir   string // Directory for this node's data
	stateFile string // Path to state.json
	logFile   string // Path to log.jsonl
	logWriter *bufio.Writer // Buffered writer for log
	logFd     *os.File      // Keep file open
}

// NewPersister creates a new persister for a given node
func NewPersister(nodeID string) *Persister {
	// Create data directory structure: data/nodeID/
	dataDir := filepath.Join("data", nodeID)
	
	// Create directory if it doesn't exist
	os.MkdirAll(dataDir, 0755)
	
	p := &Persister{
		dataDir:   dataDir,
		stateFile: filepath.Join(dataDir, "state.json"),
		logFile:   filepath.Join(dataDir, "log.jsonl"),
	}
	
	// Open log file once and keep it open
	f, err := os.OpenFile(p.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		p.logFd = f
		p.logWriter = bufio.NewWriterSize(f, 64*1024) // 64KB buffer
	}
	
	return p
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
	
	// Write to temp file first (atomic write pattern)
	tempFile := p.stateFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp state file: %v", err)
	}
	
	// Atomic rename
	if err := os.Rename(tempFile, p.stateFile); err != nil {
		return fmt.Errorf("failed to rename state file: %v", err)
	}
	
	return nil
}

// LoadState loads currentTerm and votedFor from disk
func (p *Persister) LoadState() (int, string, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Check if state file exists
	if _, err := os.Stat(p.stateFile); os.IsNotExist(err) {
		return 0, "", false, nil
	}
	
	data, err := os.ReadFile(p.stateFile)
	if err != nil {
		return 0, "", false, fmt.Errorf("failed to read state file: %v", err)
	}
	
	var state PersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return 0, "", false, fmt.Errorf("failed to unmarshal state: %v", err)
	}
	
	return state.CurrentTerm, state.VotedFor, true, nil
}

// AppendLogEntry appends a log entry using buffered writes
func (p *Persister) AppendLogEntry(entry LogEntry) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.logWriter == nil {
		return fmt.Errorf("log writer not initialized")
	}
	
	// Marshal entry
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %v", err)
	}
	
	// Write to buffer (NOT disk yet)
	if _, err := p.logWriter.WriteString(string(data) + "\n"); err != nil {
		return fmt.Errorf("failed to write log entry: %v", err)
	}
	
	// Flush buffer every 100 entries (batching)
	// This reduces syscalls dramatically
	return nil
}

// Flush forces buffered writes to disk
func (p *Persister) Flush() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.logWriter == nil {
		return nil
	}
	
	if err := p.logWriter.Flush(); err != nil {
		return err
	}
	
	// Sync to disk
	if p.logFd != nil {
		return p.logFd.Sync()
	}
	
	return nil
}

// LoadLog loads all log entries from disk
func (p *Persister) LoadLog() ([]LogEntry, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Check if log file exists
	if _, err := os.Stat(p.logFile); os.IsNotExist(err) {
		return []LogEntry{{Term: 0, Index: 0, Command: nil}}, nil
	}
	
	f, err := os.Open(p.logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}
	defer f.Close()
	
	log := []LogEntry{{Term: 0, Index: 0, Command: nil}}
	
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal log entry at line %d: %v", lineNum, err)
		}
		log = append(log, entry)
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %v", err)
	}
	
	return log, nil
}

// TruncateLog truncates and rewrites the log
func (p *Persister) TruncateLog(entries []LogEntry) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Close current file
	if p.logWriter != nil {
		p.logWriter.Flush()
	}
	if p.logFd != nil {
		p.logFd.Close()
	}
	
	// Remove old file
	os.Remove(p.logFile)
	
	// Reopen
	f, err := os.OpenFile(p.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	
	p.logFd = f
	p.logWriter = bufio.NewWriterSize(f, 64*1024)
	
	// Write all entries
	for i := 1; i < len(entries); i++ {
		data, _ := json.Marshal(entries[i])
		p.logWriter.WriteString(string(data) + "\n")
	}
	
	p.logWriter.Flush()
	p.logFd.Sync()
	
	return nil
}

// Close cleans up resources
func (p *Persister) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.logWriter != nil {
		p.logWriter.Flush()
	}
	if p.logFd != nil {
		return p.logFd.Close()
	}
	return nil
}

// GetDataDir returns the data directory
func (p *Persister) GetDataDir() string {
	return p.dataDir
}

// DeleteAll removes all persisted state
func (p *Persister) DeleteAll() error {
	p.Close()
	return os.RemoveAll(p.dataDir)
}
