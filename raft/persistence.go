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
	dataDir   string          // Directory for this node's data
	stateFile string          // Path to state.json
	logFile   string          // Path to log.jsonl
	logFD     *os.File        // Keep log file open for performance
	logWriter *bufio.Writer   // Buffered writer for batching
	disabled  bool            // If true, skip all disk I/O (for benchmarking)
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
		disabled:  false, // Persistence enabled by default
	}
	
	// Open log file for appending (keep it open)
	f, err := os.OpenFile(p.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		p.logFD = f
		p.logWriter = bufio.NewWriterSize(f, 64*1024) // 64KB buffer
	}
	
	return p
}

// DisablePersistence turns off all disk writes (for benchmarking)
func (p *Persister) DisablePersistence() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.disabled = true
}

// SaveState atomically saves currentTerm and votedFor to disk
func (p *Persister) SaveState(term int, votedFor string) error {
	if p.disabled {
		return nil // Skip if disabled
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	state := PersistentState{
		CurrentTerm: term,
		VotedFor:    votedFor,
	}
	
	// Marshal to JSON
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %v", err)
	}
	
	// Write to temp file first (atomic write pattern)
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
func (p *Persister) LoadState() (int, string, bool, error) {
	if p.disabled {
		return 0, "", false, nil // Skip if disabled
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Check if state file exists
	data, err := os.ReadFile(p.stateFile)
	if os.IsNotExist(err) {
		return 0, "", false, nil // No state file, fresh start
	}
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

// AppendLogEntry appends a single log entry to the log file (no-op if disabled)
func (p *Persister) AppendLogEntry(entry LogEntry) error {
	if p.disabled {
		return nil // Skip if disabled
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.logWriter == nil {
		return nil // Silently skip if writer not initialized
	}
	
	// Marshal entry to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %v", err)
	}
	
	// Write to buffer (no fsync, batched automatically)
	if _, err := p.logWriter.Write(data); err != nil {
		return fmt.Errorf("failed to write log entry: %v", err)
	}
	if _, err := p.logWriter.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write newline: %v", err)
	}
	
	return nil
}

// Flush forces buffered data to disk
func (p *Persister) Flush() error {
	if p.disabled {
		return nil
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.logWriter != nil {
		return p.logWriter.Flush()
	}
	return nil
}

// LoadLog loads all log entries from disk
func (p *Persister) LoadLog() ([]LogEntry, error) {
	if p.disabled {
		return []LogEntry{{Term: 0, Index: 0, Command: nil}}, nil
	}
	
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

// TruncateLog truncates and rewrites the log
func (p *Persister) TruncateLog(entries []LogEntry) error {
	if p.disabled {
		return nil
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Close existing file
	if p.logWriter != nil {
		p.logWriter.Flush()
	}
	if p.logFD != nil {
		p.logFD.Close()
	}
	
	// Remove old log file
	os.Remove(p.logFile)
	
	// Reopen file
	f, err := os.OpenFile(p.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to reopen log file: %v", err)
	}
	p.logFD = f
	p.logWriter = bufio.NewWriterSize(f, 64*1024)
	
	// Write all entries (except dummy at index 0)
	for i := 1; i < len(entries); i++ {
		data, err := json.Marshal(entries[i])
		if err != nil {
			return err
		}
		p.logWriter.Write(data)
		p.logWriter.WriteString("\n")
	}
	
	return p.logWriter.Flush()
}

// Close closes the log file
func (p *Persister) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.logWriter != nil {
		p.logWriter.Flush()
	}
	if p.logFD != nil {
		return p.logFD.Close()
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
