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
        ElectionTimeoutMin: 500 * time.Millisecond,  // Increased for stability under load
        ElectionTimeoutMax: 1000 * time.Millisecond, // Increased for stability under load
        HeartbeatInterval:  100 * time.Millisecond,  // More frequent heartbeats
        RPCTimeout:         1000 * time.Millisecond,
    }
}
