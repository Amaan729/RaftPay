package raft

import "time"

// Config holds configuration for a Raft node
type Config struct {
    ElectionTimeoutMin time.Duration
    ElectionTimeoutMax time.Duration
    HeartbeatInterval  time.Duration
    RPCTimeout         time.Duration
}

// DefaultConfig returns recommended Raft configuration
func DefaultConfig() *Config {
    return &Config{
        ElectionTimeoutMin: 300 * time.Millisecond,  // Balanced for performance
        ElectionTimeoutMax: 600 * time.Millisecond,  // Balanced for performance
        HeartbeatInterval:  75 * time.Millisecond,   // Fast heartbeats
        RPCTimeout:         500 * time.Millisecond,  // Fast RPCs
    }
}
