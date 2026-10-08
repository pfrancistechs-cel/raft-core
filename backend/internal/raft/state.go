package raft

type NodeRole string

const (
	RoleFollower  NodeRole = "FOLLOWER"
	RoleCandidate NodeRole = "CANDIDATE"
	RoleLeader    NodeRole = "LEADER"
)

type LogEntry struct {
	Index   uint64 `json:"index"`
	Term    uint64 `json:"term"`
	Command string `json:"command"`
}

type NodeStatus struct {
	ID          string   `json:"id"`
	Role        NodeRole `json:"role"`
	CurrentTerm uint64   `json:"current_term"`
	VotedFor    string   `json:"voted_for"`
	CommitIndex uint64   `json:"commit_index"`
	LastApplied uint64   `json:"last_applied"`
	IsPartitioned bool   `json:"is_partitioned"`
	LogEntries  []LogEntry `json:"log_entries"`
}

