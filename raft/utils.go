package raft

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}

func (rn *RaftNode) lastLogIndex() int {
    return len(rn.log) - 1
}

func (rn *RaftNode) lastLogTerm() int {
    if len(rn.log) == 0 {
        return 0
    }
    return rn.log[len(rn.log)-1].Term
}

func (rn *RaftNode) getLogEntry(index int) *LogEntry {
    if index < 0 || index >= len(rn.log) {
        return nil
    }
    return &rn.log[index]
}
