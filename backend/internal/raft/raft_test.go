package raft

import (
	"testing"
)

type mockTransport struct{}

func (m *mockTransport) SendRequestVote(target string, args RequestVoteArgs) (RequestVoteReply, error) {
	return RequestVoteReply{Term: args.Term, VoteGranted: true}, nil
}

func (m *mockTransport) SendAppendEntries(target string, args AppendEntriesArgs) (AppendEntriesReply, error) {
	return AppendEntriesReply{Term: args.Term, Success: true}, nil
}

func TestRaftNodeInitialState(t *testing.T) {
	tr := &mockTransport{}
	node := NewNode("node-1", []string{"node-2", "node-3"}, tr)
	status := node.GetStatus()
	if status.Role != RoleFollower {
		t.Fatalf("expected RoleFollower, got %v", status.Role)
	}
	if status.CurrentTerm != 0 {
		t.Fatalf("expected term 0, got %d", status.CurrentTerm)
	}
}
