package raft

import (
	"sync"
	"time"
)

type Transport interface {
	SendRequestVote(target string, args RequestVoteArgs) (RequestVoteReply, error)
	SendAppendEntries(target string, args AppendEntriesArgs) (AppendEntriesReply, error)
}

type Node struct {
	mu            sync.RWMutex
	id            string
	peers         []string
	role          NodeRole
	currentTerm   uint64
	votedFor      string
	log           []LogEntry
	commitIndex   uint64
	lastApplied   uint64
	nextIndex     map[string]uint64
	matchIndex    map[string]uint64
	transport     Transport
	isPartitioned bool
	lastHeartbeat time.Time
}

func NewNode(id string, peers []string, transport Transport) *Node {
	n := &Node{
		id:            id,
		peers:         peers,
		role:          RoleFollower,
		currentTerm:   0,
		votedFor:      "",
		log:           make([]LogEntry, 0),
		commitIndex:   0,
		lastApplied:   0,
		nextIndex:     make(map[string]uint64),
		matchIndex:    make(map[string]uint64),
		transport:     transport,
		lastHeartbeat: time.Now(),
	}

	// 1-based log indexing: index 0 is sentinel
	n.log = append(n.log, LogEntry{Index: 0, Term: 0, Command: "SENTINEL"})
	return n
}

func (n *Node) ID() string {
	return n.id
}

func (n *Node) SetPartitioned(val bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.isPartitioned = val
}

func (n *Node) HandleRequestVote(args RequestVoteArgs) RequestVoteReply {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.isPartitioned {
		return RequestVoteReply{Term: n.currentTerm, VoteGranted: false}
	}

	// If term is outdated, reject
	if args.Term < n.currentTerm {
		return RequestVoteReply{Term: n.currentTerm, VoteGranted: false}
	}

	// If term is newer, step down to follower
	if args.Term > n.currentTerm {
		n.currentTerm = args.Term
		n.role = RoleFollower
		n.votedFor = ""
	}

	lastLog := n.log[len(n.log)-1]
	upToDate := false
	if args.LastLogTerm > lastLog.Term {
		upToDate = true
	} else if args.LastLogTerm == lastLog.Term && args.LastLogIndex >= lastLog.Index {
		upToDate = true
	}

	canVote := (n.votedFor == "" || n.votedFor == args.CandidateID) && upToDate
	if canVote {
		n.votedFor = args.CandidateID
		n.lastHeartbeat = time.Now()
		return RequestVoteReply{Term: n.currentTerm, VoteGranted: true}
	}

	return RequestVoteReply{Term: n.currentTerm, VoteGranted: false}
}

func (n *Node) HandleAppendEntries(args AppendEntriesArgs) AppendEntriesReply {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.isPartitioned {
		return AppendEntriesReply{Term: n.currentTerm, Success: false}
	}

	if args.Term < n.currentTerm {
		return AppendEntriesReply{Term: n.currentTerm, Success: false}
	}

	// Valid leader recognized
	n.lastHeartbeat = time.Now()
	if args.Term > n.currentTerm || n.role != RoleFollower {
		n.currentTerm = args.Term
		n.role = RoleFollower
		n.votedFor = args.LeaderID
	}

	// Log matching invariant: prevLogIndex & prevLogTerm must match
	if args.PrevLogIndex >= uint64(len(n.log)) || n.log[args.PrevLogIndex].Term != args.PrevLogTerm {
		return AppendEntriesReply{Term: n.currentTerm, Success: false, MatchIndex: uint64(len(n.log) - 1)}
	}

	// Append any new entries not already in the log
	insertIndex := args.PrevLogIndex + 1
	for _, entry := range args.Entries {
		if insertIndex < uint64(len(n.log)) {
			if n.log[insertIndex].Term != entry.Term {
				n.log = n.log[:insertIndex]
				n.log = append(n.log, entry)
			}
		} else {
			n.log = append(n.log, entry)
		}
		insertIndex++
	}

	// Update commit index
	if args.LeaderCommit > n.commitIndex {
		lastNewIndex := uint64(len(n.log) - 1)
		if args.LeaderCommit < lastNewIndex {
			n.commitIndex = args.LeaderCommit
		} else {
			n.commitIndex = lastNewIndex
		}
		n.lastApplied = n.commitIndex
	}

	return AppendEntriesReply{Term: n.currentTerm, Success: true, MatchIndex: uint64(len(n.log) - 1)}
}

func (n *Node) StartElection() {
	n.mu.Lock()
	if n.isPartitioned {
		n.mu.Unlock()
		return
	}

	n.role = RoleCandidate
	n.currentTerm++
	n.votedFor = n.id
	lastLog := n.log[len(n.log)-1]

	args := RequestVoteArgs{
		Term:         n.currentTerm,
		CandidateID:  n.id,
		LastLogIndex: lastLog.Index,
		LastLogTerm:  lastLog.Term,
	}
	votesReceived := 1
	term := n.currentTerm
	peers := append([]string{}, n.peers...)
	n.mu.Unlock()

	for _, peer := range peers {
		reply, err := n.transport.SendRequestVote(peer, args)
		if err == nil && reply.VoteGranted {
			n.mu.Lock()
			if n.currentTerm == term && n.role == RoleCandidate {
				votesReceived++
				// Majority quorum check: (N/2) + 1
				quorum := (len(peers)+1)/2 + 1
				if votesReceived >= quorum {
					n.role = RoleLeader
					for _, p := range peers {
						n.nextIndex[p] = uint64(len(n.log))
						n.matchIndex[p] = 0
					}
					n.mu.Unlock()
					n.BroadcastHeartbeat()
					return
				}
			}
			n.mu.Unlock()
		}
	}
}

func (n *Node) BroadcastHeartbeat() {
	n.mu.RLock()
	if n.role != RoleLeader || n.isPartitioned {
		n.mu.RUnlock()
		return
	}
	peers := append([]string{}, n.peers...)
	term := n.currentTerm
	leaderID := n.id
	commitIdx := n.commitIndex
	n.mu.RUnlock()

	for _, peer := range peers {
		n.mu.RLock()
		prevIdx := n.nextIndex[peer] - 1
		if prevIdx >= uint64(len(n.log)) {
			prevIdx = uint64(len(n.log) - 1)
		}
		prevTerm := n.log[prevIdx].Term
		entriesToSend := make([]LogEntry, 0)
		if uint64(len(n.log)) > n.nextIndex[peer] {
			entriesToSend = append(entriesToSend, n.log[n.nextIndex[peer]:]...)
		}
		n.mu.RUnlock()

		args := AppendEntriesArgs{
			Term:         term,
			LeaderID:     leaderID,
			PrevLogIndex: prevIdx,
			PrevLogTerm:  prevTerm,
			Entries:      entriesToSend,
			LeaderCommit: commitIdx,
		}

		go func(target string, req AppendEntriesArgs) {
			reply, err := n.transport.SendAppendEntries(target, req)
			if err == nil && reply.Success {
				n.mu.Lock()
				n.nextIndex[target] = reply.MatchIndex + 1
				n.matchIndex[target] = reply.MatchIndex

				// Check if entry can be committed
				for idx := n.commitIndex + 1; idx < uint64(len(n.log)); idx++ {
					if n.log[idx].Term == n.currentTerm {
						count := 1 // self
						for _, p := range n.peers {
							if n.matchIndex[p] >= idx {
								count++
							}
						}
						if count >= (len(n.peers)+1)/2+1 {
							n.commitIndex = idx
							n.lastApplied = idx
						}
					}
				}
				n.mu.Unlock()
			}
		}(peer, args)
	}
}

func (n *Node) Propose(command string) (uint64, uint64, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.role != RoleLeader || n.isPartitioned {
		return 0, 0, false
	}

	newIndex := uint64(len(n.log))
	entry := LogEntry{
		Index:   newIndex,
		Term:    n.currentTerm,
		Command: command,
	}
	n.log = append(n.log, entry)
	n.matchIndex[n.id] = newIndex

	go n.BroadcastHeartbeat()
	return newIndex, n.currentTerm, true
}

func (n *Node) Status() NodeStatus {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// omit sentinel from returned slice for clean visualization
	entries := make([]LogEntry, 0)
	if len(n.log) > 1 {
		entries = append(entries, n.log[1:]...)
	}

	return NodeStatus{
		ID:            n.id,
		Role:          n.role,
		CurrentTerm:   n.currentTerm,
		VotedFor:      n.votedFor,
		CommitIndex:   n.commitIndex,
		LastApplied:   n.lastApplied,
		IsPartitioned: n.isPartitioned,
		LogEntries:    entries,
	}
}


<!-- // Clamped election jitter between 150ms-300ms to avoid split votes
 -->

<!-- // RequestVote term update -->

<!-- // AppendEntries log replication -->

<!-- // Replication batching optimization -->

<!-- // Election timeout jitter -->

<!-- // Lock cleanup -->
