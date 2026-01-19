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
        ElectionTimeoutMin: 150 * time.Millisecond,
        ElectionTimeoutMax: 300 * time.Millisecond,
        HeartbeatInterval:  50 * time.Millisecond,
        RPCTimeout:         100 * time.Millisecond,
    }
}
