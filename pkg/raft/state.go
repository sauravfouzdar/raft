package raft

import (
	"sync"
)

// NodeState represents the state of a Raft node
type NodeState int

const (
	// Follower state - receives log entries from leader
	Follower NodeState = iota
	// Candidate state - requests votes from other nodes
	Candidate
	// Leader state - handles client requests and replicates log entries
	Leader
)

// String returns a string representation of the NodeState
func (s NodeState) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	default:
		return "Unknown"
	}
}

// RaftState represents the persistent state of a Raft node
type RaftState struct {
	// CurrentTerm is the latest term the server has seen
	CurrentTerm uint64

	// VotedFor is the candidate that received vote in current term (or empty)
	VotedFor string

	// mu protects access to the state
	mu sync.RWMutex
}

// NewRaftState creates a new RaftState with initial values
func NewRaftState() *RaftState {
	return &RaftState{
		CurrentTerm: 0,
		VotedFor:    "",
	}
}

// GetCurrentTerm returns the current term
func (rs *RaftState) GetCurrentTerm() uint64 {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.CurrentTerm
}

// SetCurrentTerm sets the current term
func (rs *RaftState) SetCurrentTerm(term uint64) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.CurrentTerm = term
	// Reset votedFor if term changes
	if term > rs.CurrentTerm {
		rs.VotedFor = ""
	}
}

// GetVotedFor returns the candidate that received vote in current term
func (rs *RaftState) GetVotedFor() string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.VotedFor
}

// SetVotedFor sets the candidate that received vote in current term
func (rs *RaftState) SetVotedFor(candidateID string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.VotedFor = candidateID
}

// VolatileState represents the volatile state of a Raft node
type VolatileState struct {
	// CommitIndex is the highest log entry known to be committed
	CommitIndex uint64

	// LastApplied is the highest log entry applied to state machine
	LastApplied uint64

	// State is the current node state (follower, candidate, or leader)
	State NodeState

	// mu protects access to the state
	mu sync.RWMutex
}

// NewVolatileState creates a new VolatileState with initial values
func NewVolatileState() *VolatileState {
	return &VolatileState{
		CommitIndex: 0,
		LastApplied: 0,
		State:       Follower,
	}
}

// GetCommitIndex returns the commit index
func (vs *VolatileState) GetCommitIndex() uint64 {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.CommitIndex
}

// SetCommitIndex sets the commit index
func (vs *VolatileState) SetCommitIndex(index uint64) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.CommitIndex = index
}

// GetLastApplied returns the last applied index
func (vs *VolatileState) GetLastApplied() uint64 {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.LastApplied
}

// SetLastApplied sets the last applied index
func (vs *VolatileState) SetLastApplied(index uint64) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.LastApplied = index
}

// GetState returns the current node state
func (vs *VolatileState) GetState() NodeState {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.State
}

// SetState sets the current node state
func (vs *VolatileState) SetState(state NodeState) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.State = state
}

// LeaderState represents the state maintained by the leader
type LeaderState struct {
	// NextIndex for each server, index of the next log entry to send
	NextIndex map[string]uint64

	// MatchIndex for each server, index of highest log entry known to be replicated
	MatchIndex map[string]uint64

	// mu protects access to the state
	mu sync.RWMutex
}

// NewLeaderState creates a new LeaderState with initial values
func NewLeaderState(peers []string, lastLogIndex uint64) *LeaderState {
	nextIndex := make(map[string]uint64)
	matchIndex := make(map[string]uint64)

	for _, peer := range peers {
		nextIndex[peer] = lastLogIndex + 1
		matchIndex[peer] = 0
	}

	return &LeaderState{
		NextIndex:  nextIndex,
		MatchIndex: matchIndex,
	}
}

// GetNextIndex returns the next index for a peer
func (ls *LeaderState) GetNextIndex(peer string) uint64 {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.NextIndex[peer]
}

// SetNextIndex sets the next index for a peer
func (ls *LeaderState) SetNextIndex(peer string, index uint64) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.NextIndex[peer] = index
}

// GetMatchIndex returns the match index for a peer
func (ls *LeaderState) GetMatchIndex(peer string) uint64 {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.MatchIndex[peer]
}

// SetMatchIndex sets the match index for a peer
func (ls *LeaderState) SetMatchIndex(peer string, index uint64) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.MatchIndex[peer] = index
}