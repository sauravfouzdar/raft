package raft

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sauravfouzdar/raft/pkg/config"
	"github.com/sauravfouzdar/raft/pkg/logger"
	"github.com/sauravfouzdar/raft/pkg/protocol"
)

// ApplyFunc is a function that applies a committed log entry to the state machine
type ApplyFunc func(entry protocol.LogEntry) error

// Node represents a Raft consensus node
type Node struct {
	// Config contains the node configuration
	Config *config.Config

	// ID is the unique identifier for this node
	ID string

	// Logger is used for logging
	Logger *logger.Logger

	// Transport is used for network communication
	Transport protocol.Transport

	// Storage is used for persistent state
	Storage Storage

	// RaftState contains the persistent state
	RaftState *RaftState

	// VolatileState contains the volatile state
	VolatileState *VolatileState

	// LeaderState contains the leader-specific state
	LeaderState *LeaderState

	// Log contains the log entries
	Log *Log

	// Timer manages election and heartbeat timeouts
	Timer *Timer

	// LastContact is the last time we received contact from the leader
	LastContact time.Time

	// ApplyFunc is called when a log entry is committed
	ApplyFunc ApplyFunc

	// VotesReceived is the number of votes received in an election
	VotesReceived int

	// ctx is the context for cancellation
	ctx context.Context

	// cancel cancels the context
	cancel context.CancelFunc

	// mu protects access to the node
	mu sync.RWMutex

	// running indicates if the node is running
	running bool
}

// NewNode creates a new Raft node
func NewNode(config *config.Config, applyFunc ApplyFunc) (*Node, error) {
	// Create logger
	log := logger.New(config.NodeID, config.LogLevel)
	log.Info("Creating new Raft node with ID: %s", config.NodeID)

	// Get peer node IDs (excluding self)
	peerIDs := make([]string, 0, len(config.Peers))
	for id := range config.Peers {
		if id != config.NodeID {
			peerIDs = append(peerIDs, id)
		}
	}

	// Create storage
	storage, err := NewFileStorage(config.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %v", err)
	}

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Create node
	n := &Node{
		Config:        config,
		ID:            config.NodeID,
		Logger:        log,
		Storage:       storage,
		RaftState:     NewRaftState(),
		VolatileState: NewVolatileState(),
		Log:           NewLog(),
		Timer:         NewTimer(config.ElectionTimeoutMin, config.ElectionTimeoutMax, config.HeartbeatTimeout),
		LastContact:   time.Now(),
		ApplyFunc:     applyFunc,
		ctx:           ctx,
		cancel:        cancel,
		running:       false,
	}

	return n, nil
}

// SetTransport sets the transport for the node
func (n *Node) SetTransport(transport protocol.Transport) {
	n.Transport = transport
}

// Start starts the Raft node
func (n *Node) Start() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.running {
		return fmt.Errorf("node already running")
	}

	// Check if transport is set
	if n.Transport == nil {
		return fmt.Errorf("transport not set")
	}

	n.Logger.Info("Starting Raft node")

	// Load persistent state
	term, votedFor, err := n.Storage.LoadState()
	if err != nil {
		n.Logger.Error("Failed to load state: %v", err)
		return fmt.Errorf("failed to load state: %v", err)
	}

	n.RaftState.SetCurrentTerm(term)
	n.RaftState.SetVotedFor(votedFor)

	// Load log entries
	entries, err := n.Storage.LoadLogEntries()
	if err != nil {
		n.Logger.Error("Failed to load log entries: %v", err)
		return fmt.Errorf("failed to load log entries: %v", err)
	}

	// TODO: Properly restore log entries
	n.Logger.Info("Loaded %d log entries", len(entries))

	// Start transport
	if err := n.Transport.Start(); err != nil {
		n.Logger.Error("Failed to start transport: %v", err)
		return fmt.Errorf("failed to start transport: %v", err)
	}

	// Start main loop
	go n.run()

	n.running = true
	n.Logger.Info("Raft node started")
	return nil
}

// Stop stops the Raft node
func (n *Node) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.running {
		return nil
	}

	n.Logger.Info("Stopping Raft node")

	// Cancel context
	n.cancel()

	// Stop transport
	if err := n.Transport.Stop(); err != nil {
		n.Logger.Error("Failed to stop transport: %v", err)
	}

	// Stop timer
	n.Timer.Stop()

	// Close storage
	if err := n.Storage.Close(); err != nil {
		n.Logger.Error("Failed to close storage: %v", err)
	}

	n.running = false
	n.Logger.Info("Raft node stopped")
	return nil
}

// run is the main loop for the Raft node
func (n *Node) run() {
	// Start as a follower
	n.VolatileState.SetState(Follower)
	n.Timer.ResetElection()

	for {
		state := n.VolatileState.GetState()
		n.Logger.Debug("Current state: %s, Term: %d", state, n.RaftState.GetCurrentTerm())

		switch state {
		case Follower:
			n.runFollower()
		case Candidate:
			n.runCandidate()
		case Leader:
			n.runLeader()
		}

		// Check if context was cancelled
		select {
		case <-n.ctx.Done():
			return
		default:
			// Continue running
		}
	}
}

// sendHeartbeats sends AppendEntries RPCs with no entries to all peers
func (n *Node) sendHeartbeats() {
	n.Logger.Debug("Sending heartbeats for term %d", n.RaftState.GetCurrentTerm())

	for peerID := range n.Config.Peers {
		if peerID == n.ID {
			continue // Skip self
		}

		go n.sendAppendEntries(peerID, true)
	}
}

// sendAppendEntries sends an AppendEntries RPC to a peer
func (n *Node) sendAppendEntries(peerID string, isHeartbeat bool) {
	nextIndex := n.LeaderState.GetNextIndex(peerID)
	prevLogIndex := nextIndex - 1
	
	// Get prevLogTerm
	var prevLogTerm uint64 = 0
	if prevLogIndex > 0 {
		prevLogEntry, err := n.Log.GetEntry(prevLogIndex)
		if err != nil {
			n.Logger.Error("Failed to get log entry at index %d: %v", prevLogIndex, err)
			return
		}
		prevLogTerm = prevLogEntry.Term
	}

	// Prepare entries to send
	var entries []protocol.LogEntry
	if !isHeartbeat {
		var err error
		logEntries, err := n.Log.GetEntries(nextIndex, 100) // Limit to 100 entries per batch
		if err != nil {
			n.Logger.Error("Failed to get log entries: %v", err)
			return
		}
		
		// Convert to protocol.LogEntry
		for _, entry := range logEntries {
			entries = append(entries, protocol.LogEntry{
				Index: entry.Index,
				Term: entry.Term,
				Type: protocol.LogEntryType(entry.Type),
				Command: entry.Command,
			})
		}
	}

	// Create AppendEntries args
	args := &protocol.AppendEntriesArgs{
		Term:         n.RaftState.GetCurrentTerm(),
		LeaderID:     n.ID,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      entries,
		LeaderCommit: n.VolatileState.GetCommitIndex(),
	}

	// Serialize arguments
	data, err := protocol.SerializeAppendEntriesArgs(args)
	if err != nil {
		n.Logger.Error("Failed to serialize AppendEntries args: %v", err)
		return
	}

	// Create RPC message
	msg := &protocol.RPCMessage{
		Type: protocol.AppendEntriesRequest,
		From: n.ID,
		To:   peerID,
		Data: data,
	}

	// Send message
	if err := n.Transport.SendMessage(msg); err != nil {
		n.Logger.Error("Failed to send AppendEntries to %s: %v", peerID, err)
	} else {
		if isHeartbeat {
			n.Logger.Debug("Sent heartbeat to %s for term %d", peerID, args.Term)
		} else {
			n.Logger.Debug("Sent AppendEntries to %s for term %d with %d entries", 
				peerID, args.Term, len(entries))
		}
	}
}

// runFollower runs the follower state logic
func (n *Node) runFollower() {
	n.Logger.Info("Entering follower state")

	// Reset election timer
	n.Timer.ResetElection()

	for n.VolatileState.GetState() == Follower {
		select {
		case <-n.ctx.Done():
			return

		case <-n.Timer.ElectionTimeout():
			n.Logger.Info("Election timeout, converting to candidate")
			n.VolatileState.SetState(Candidate)
			return

		case msg := <-n.Transport.Messages():
			n.handleRPCMessage(msg)
		}
	}
}

// runCandidate runs the candidate state logic
func (n *Node) runCandidate() {
	n.Logger.Info("Entering candidate state")

	// Increment current term and vote for self
	currentTerm := n.RaftState.GetCurrentTerm() + 1
	n.RaftState.SetCurrentTerm(currentTerm)
	n.RaftState.SetVotedFor(n.ID)

	// Save state
	if err := n.Storage.SaveState(currentTerm, n.ID); err != nil {
		n.Logger.Error("Failed to save state: %v", err)
	}

	// Reset votes received
	n.VotesReceived = 1 // Vote for self

	// Reset election timer
	n.Timer.ResetElection()

	// Send RequestVote RPCs to all peers
	n.startElection()

	// Wait for votes or timeout
	for n.VolatileState.GetState() == Candidate {
		select {
		case <-n.ctx.Done():
			return

		case <-n.Timer.ElectionTimeout():
			n.Logger.Info("Election timeout, starting new election")
			return // This will restart the candidate state loop

		case msg := <-n.Transport.Messages():
			n.handleRPCMessage(msg)
		}
	}
}

// runLeader runs the leader state logic
func (n *Node) runLeader() {
	n.Logger.Info("Entering leader state")

	// Initialize leader state
	lastLogEntry := n.Log.GetLastEntry()
	peerIDs := make([]string, 0, len(n.Config.Peers))
	for id := range n.Config.Peers {
		if id != n.ID {
			peerIDs = append(peerIDs, id)
		}
	}
	n.LeaderState = NewLeaderState(peerIDs, lastLogEntry.Index)

	// Append a no-op entry as first entry of leader's term
	n.Log.Append(n.RaftState.GetCurrentTerm(), NoOpEntry, nil)

	// Reset heartbeat timer
	n.Timer.ResetHeartbeat()

	// Send initial heartbeats
	n.sendHeartbeats()

	for n.VolatileState.GetState() == Leader {
		select {
		case <-n.ctx.Done():
			return

		case <-n.Timer.HeartbeatTimeout():
			n.sendHeartbeats()
			n.Timer.ResetHeartbeat()

		case msg := <-n.Transport.Messages():
			n.handleRPCMessage(msg)
		}
	}
}

// handleRPCMessage handles an incoming RPC message
func (n *Node) handleRPCMessage(msg *protocol.RPCMessage) {
	switch msg.Type {
	case protocol.AppendEntriesRequest:
		n.handleAppendEntriesRequest(msg)
	case protocol.AppendEntriesResponse:
		n.handleAppendEntriesResponse(msg)
	case protocol.RequestVoteRequest:
		n.handleRequestVoteRequest(msg)
	case protocol.RequestVoteResponse:
		n.handleRequestVoteResponse(msg)
	default:
		n.Logger.Error("Unknown RPC message type: %d", msg.Type)
	}
}

// handleAppendEntriesRequest handles an AppendEntries RPC request
func (n *Node) handleAppendEntriesRequest(msg *protocol.RPCMessage) {
	// Deserialize request
	args, err := protocol.DeserializeAppendEntriesArgs(msg.Data)
	if err != nil {
		n.Logger.Error("Failed to deserialize AppendEntries args: %v", err)
		return
	}

	// Prepare response
	reply := &protocol.AppendEntriesReply{
		Term:      n.RaftState.GetCurrentTerm(),
		Success:   false,
		NextIndex: 0,
	}

	// Check if message is from current term
	if args.Term < n.RaftState.GetCurrentTerm() {
		n.Logger.Debug("Rejecting AppendEntries from %s with older term %d (current: %d)",
			msg.From, args.Term, n.RaftState.GetCurrentTerm())
		n.sendAppendEntriesResponse(msg.From, reply)
		return
	}

	// If we receive an AppendEntries from a newer term, update term and convert to follower
	if args.Term > n.RaftState.GetCurrentTerm() {
		n.Logger.Info("Converting to follower due to newer term %d (was %d)",
			args.Term, n.RaftState.GetCurrentTerm())
		n.RaftState.SetCurrentTerm(args.Term)
		n.RaftState.SetVotedFor("")
		n.VolatileState.SetState(Follower)
		
		// Save state
		if err := n.Storage.SaveState(args.Term, ""); err != nil {
			n.Logger.Error("Failed to save state: %v", err)
		}
	}

	// Reset election timer since we received an AppendEntries from the leader
	n.Timer.ResetElection()
	n.LastContact = time.Now()

	// Check log consistency
	logOK := true
	if args.PrevLogIndex > 0 {
		// Check if log contains entry at PrevLogIndex
		entry, err := n.Log.GetEntry(args.PrevLogIndex)
		if err != nil || entry.Term != args.PrevLogTerm {
			logOK = false
			
			// Suggest next index for faster convergence
			if err != nil {
				// Log doesn't have PrevLogIndex, suggest last index + 1
				reply.NextIndex = n.Log.GetLastEntry().Index + 1
			} else {
				// Log has entry with different term, suggest first index of that term
				// This is an optimization to speed up log consistency
				for i := args.PrevLogIndex - 1; i > 0; i-- {
					entry, err := n.Log.GetEntry(i)
					if err != nil || entry.Term != args.PrevLogTerm {
						reply.NextIndex = i + 1
						break
					}
				}
			}
		}
	}

	if logOK {
		// If we have existing entries that conflict with new entries, delete them
		if args.PrevLogIndex+1 <= n.Log.GetLastEntry().Index {
			n.Log.DeleteEntriesFrom(args.PrevLogIndex + 1)
		}

		// Append new entries if any
		if len(args.Entries) > 0 {
			for _, protocolEntry := range args.Entries {
				// Convert protocol.LogEntry to raft.LogEntry
				entry := LogEntry{
					Index: protocolEntry.Index,
					Term: protocolEntry.Term,
					Type: LogEntryType(protocolEntry.Type),
					Command: protocolEntry.Command,
				}
				
				n.Log.Append(entry.Term, entry.Type, entry.Command)
				
				// Save to storage
				if err := n.Storage.SaveLogEntry(entry); err != nil {
					n.Logger.Error("Failed to save log entry: %v", err)
				}
			}
		}

		// Update commit index if leader has committed entries
		if args.LeaderCommit > n.VolatileState.GetCommitIndex() {
			n.updateCommitIndex(args.LeaderCommit)
		}

		reply.Success = true
		reply.NextIndex = args.PrevLogIndex + uint64(len(args.Entries)) + 1
	}

	// Send response
	n.sendAppendEntriesResponse(msg.From, reply)
}

// handleAppendEntriesResponse handles an AppendEntries RPC response
func (n *Node) handleAppendEntriesResponse(msg *protocol.RPCMessage) {
	// Only leaders should receive AppendEntries responses
	if n.VolatileState.GetState() != Leader {
		n.Logger.Warn("Received AppendEntries response as non-leader")
		return
	}

	// Deserialize response
	reply, err := protocol.DeserializeAppendEntriesReply(msg.Data)
	if err != nil {
		n.Logger.Error("Failed to deserialize AppendEntries reply: %v", err)
		return
	}

	// If the response is from a newer term, update term and convert to follower
	if reply.Term > n.RaftState.GetCurrentTerm() {
		n.Logger.Info("Converting to follower due to newer term %d in AppendEntries response (was %d)",
			reply.Term, n.RaftState.GetCurrentTerm())
		n.RaftState.SetCurrentTerm(reply.Term)
		n.RaftState.SetVotedFor("")
		n.VolatileState.SetState(Follower)
		
		// Save state
		if err := n.Storage.SaveState(reply.Term, ""); err != nil {
			n.Logger.Error("Failed to save state: %v", err)
		}
		
		return
	}

	// If AppendEntries was successful, update nextIndex and matchIndex
	if reply.Success {
		n.LeaderState.SetNextIndex(msg.From, reply.NextIndex)
		n.LeaderState.SetMatchIndex(msg.From, reply.NextIndex-1)
		
		// Check if we can commit more entries
		n.updateLeaderCommit()
	} else {
		// If AppendEntries failed because of log inconsistency, decrement nextIndex and retry
		nextIndex := n.LeaderState.GetNextIndex(msg.From)
		if reply.NextIndex > 0 && reply.NextIndex < nextIndex {
			// Use hint from follower if available
			n.LeaderState.SetNextIndex(msg.From, reply.NextIndex)
		} else {
			// Otherwise, just decrement
			n.LeaderState.SetNextIndex(msg.From, nextIndex-1)
		}
		
		// Retry AppendEntries with earlier entries
		go n.sendAppendEntries(msg.From, false)
	}
}

// handleRequestVoteRequest handles a RequestVote RPC request
func (n *Node) handleRequestVoteRequest(msg *protocol.RPCMessage) {
	// Deserialize request
	args, err := protocol.DeserializeRequestVoteArgs(msg.Data)
	if err != nil {
		n.Logger.Error("Failed to deserialize RequestVote args: %v", err)
		return
	}

	// Prepare response
	reply := &protocol.RequestVoteReply{
		Term:        n.RaftState.GetCurrentTerm(),
		VoteGranted: false,
	}

	// Check if message is from current term
	if args.Term < n.RaftState.GetCurrentTerm() {
		n.Logger.Debug("Rejecting vote for %s with older term %d (current: %d)",
			msg.From, args.Term, n.RaftState.GetCurrentTerm())
		n.sendRequestVoteResponse(msg.From, reply)
		return
	}

	// If we receive a RequestVote from a newer term, update term and convert to follower
	if args.Term > n.RaftState.GetCurrentTerm() {
		n.Logger.Info("Converting to follower due to newer term %d in RequestVote (was %d)",
			args.Term, n.RaftState.GetCurrentTerm())
		n.RaftState.SetCurrentTerm(args.Term)
		n.RaftState.SetVotedFor("")
		n.VolatileState.SetState(Follower)
		
		// Save state
		if err := n.Storage.SaveState(args.Term, ""); err != nil {
			n.Logger.Error("Failed to save state: %v", err)
		}
	}

	// Check if we can vote for this candidate
	votedFor := n.RaftState.GetVotedFor()
	canVote := votedFor == "" || votedFor == args.CandidateID

	// Check if candidate's log is at least as up-to-date as ours
	lastLogEntry := n.Log.GetLastEntry()
	logOK := args.LastLogTerm > lastLogEntry.Term ||
		(args.LastLogTerm == lastLogEntry.Term && args.LastLogIndex >= lastLogEntry.Index)

	if canVote && logOK {
		n.Logger.Info("Granting vote to %s for term %d", args.CandidateID, args.Term)
		reply.VoteGranted = true
		n.RaftState.SetVotedFor(args.CandidateID)
		
		// Save state
		if err := n.Storage.SaveState(n.RaftState.GetCurrentTerm(), args.CandidateID); err != nil {
			n.Logger.Error("Failed to save state: %v", err)
		}
		
		// Reset election timer when we grant a vote
		n.Timer.ResetElection()
	} else {
		n.Logger.Info("Rejecting vote for %s for term %d (canVote: %v, logOK: %v)",
			args.CandidateID, args.Term, canVote, logOK)
	}

	// Send response
	n.sendRequestVoteResponse(msg.From, reply)
}

// handleRequestVoteResponse handles a RequestVote RPC response
func (n *Node) handleRequestVoteResponse(msg *protocol.RPCMessage) {
	// Only candidates should receive RequestVote responses
	if n.VolatileState.GetState() != Candidate {
		n.Logger.Warn("Received RequestVote response as non-candidate")
		return
	}

	// Deserialize response
	reply, err := protocol.DeserializeRequestVoteReply(msg.Data)
	if err != nil {
		n.Logger.Error("Failed to deserialize RequestVote reply: %v", err)
		return
	}

	// If the response is from a newer term, update term and convert to follower
	if reply.Term > n.RaftState.GetCurrentTerm() {
		n.Logger.Info("Converting to follower due to newer term %d in RequestVote response (was %d)",
			reply.Term, n.RaftState.GetCurrentTerm())
		n.RaftState.SetCurrentTerm(reply.Term)
		n.RaftState.SetVotedFor("")
		n.VolatileState.SetState(Follower)
		
		// Save state
		if err := n.Storage.SaveState(reply.Term, ""); err != nil {
			n.Logger.Error("Failed to save state: %v", err)
		}
		
		return
	}

	// If vote was granted, increment votes received
	if reply.VoteGranted {
		n.VotesReceived++
		n.Logger.Info("Received vote from %s, total votes: %d", msg.From, n.VotesReceived)

		// Check if we have a majority
		peerCount := len(n.Config.Peers)
		majority := (peerCount / 2) + 1

		if n.VotesReceived >= majority {
			n.Logger.Info("Won election with %d votes out of %d", n.VotesReceived, peerCount)
			n.VolatileState.SetState(Leader)
		}
	}
}

// sendAppendEntriesResponse sends an AppendEntries RPC response
func (n *Node) sendAppendEntriesResponse(peerID string, reply *protocol.AppendEntriesReply) {
	// Serialize response
	data, err := protocol.SerializeAppendEntriesReply(reply)
	if err != nil {
		n.Logger.Error("Failed to serialize AppendEntries reply: %v", err)
		return
	}

	// Create RPC message
	msg := &protocol.RPCMessage{
		Type: protocol.AppendEntriesResponse,
		From: n.ID,
		To:   peerID,
		Data: data,
	}

	// Send message
	if err := n.Transport.SendMessage(msg); err != nil {
		n.Logger.Error("Failed to send AppendEntries response to %s: %v", peerID, err)
	}
}

// sendRequestVoteResponse sends a RequestVote RPC response
func (n *Node) sendRequestVoteResponse(peerID string, reply *protocol.RequestVoteReply) {
	// Serialize response
	data, err := protocol.SerializeRequestVoteReply(reply)
	if err != nil {
		n.Logger.Error("Failed to serialize RequestVote reply: %v", err)
		return
	}

	// Create RPC message
	msg := &protocol.RPCMessage{
		Type: protocol.RequestVoteResponse,
		From: n.ID,
		To:   peerID,
		Data: data,
	}

	// Send message
	if err := n.Transport.SendMessage(msg); err != nil {
		n.Logger.Error("Failed to send RequestVote response to %s: %v", peerID, err)
	}
}

// updateCommitIndex updates the commit index for a follower
func (n *Node) updateCommitIndex(leaderCommit uint64) {
	// Set commit index to min(leaderCommit, index of last new entry)
	lastLogIndex := n.Log.GetLastEntry().Index
	commitIndex := leaderCommit
	if leaderCommit > lastLogIndex {
		commitIndex = lastLogIndex
	}
	
	n.VolatileState.SetCommitIndex(commitIndex)
	
	// Apply newly committed entries
	n.applyCommittedEntries()
}

// updateLeaderCommit checks if we can commit more entries as a leader
func (n *Node) updateLeaderCommit() {
	// Only leaders should update commit index based on match indices
	if n.VolatileState.GetState() != Leader {
		return
	}
	
	// Get current commit index
	currentCommitIndex := n.VolatileState.GetCommitIndex()
	
	// Get last log index
	lastLogIndex := n.Log.GetLastEntry().Index
	
	// Check each index from current commit index + 1 to last log index
	for index := currentCommitIndex + 1; index <= lastLogIndex; index++ {
		// Count how many nodes have this entry
		count := 1 // Leader has it
		
		for peerID := range n.Config.Peers {
			if peerID == n.ID {
				continue // Skip self
			}
			
			if n.LeaderState.GetMatchIndex(peerID) >= index {
				count++
			}
		}
		
		// Check if we have a majority
		majority := (len(n.Config.Peers) / 2) + 1
		
		// Only commit if entry is from current term (Raft safety guarantee)
		entry, err := n.Log.GetEntry(index)
		if err != nil {
			n.Logger.Error("Failed to get log entry at index %d: %v", index, err)
			continue
		}
		
		if count >= majority && entry.Term == n.RaftState.GetCurrentTerm() {
			n.VolatileState.SetCommitIndex(index)
			
			// Apply newly committed entries
			n.applyCommittedEntries()
			
			// We've updated the commit index, return
			return
		}
	}
}

// applyCommittedEntries applies committed but not yet applied entries to the state machine
func (n *Node) applyCommittedEntries() {
	commitIndex := n.VolatileState.GetCommitIndex()
	lastApplied := n.VolatileState.GetLastApplied()
	
	// Apply all entries between lastApplied and commitIndex
	for i := lastApplied + 1; i <= commitIndex; i++ {
		entry, err := n.Log.GetEntry(i)
		if err != nil {
			n.Logger.Error("Failed to get log entry at index %d: %v", i, err)
			continue
		}
		
		// Skip no-op entries
		if entry.Type == NoOpEntry {
			n.Logger.Debug("Skipping no-op entry at index %d", i)
			n.VolatileState.SetLastApplied(i)
			continue
		}
		
		// Apply entry to state machine
		if n.ApplyFunc != nil {
			// Convert to protocol.LogEntry
			protocolEntry := protocol.LogEntry{
				Index:   entry.Index,
				Term:    entry.Term,
				Type:    protocol.LogEntryType(entry.Type),
				Command: entry.Command,
			}
			
			if err := n.ApplyFunc(protocolEntry); err != nil {
				n.Logger.Error("Failed to apply log entry at index %d: %v", i, err)
				continue
			}
		}
		
		n.Logger.Debug("Applied log entry at index %d: %s", i, entry.String())
		n.VolatileState.SetLastApplied(i)
	}
}

// Submit submits a command to the Raft cluster
func (n *Node) Submit(command []byte) (uint64, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	// Only leaders can submit commands
	if n.VolatileState.GetState() != Leader {
		return 0, fmt.Errorf("not leader")
	}
	
	// Append to log
	index, err := n.Log.Append(n.RaftState.GetCurrentTerm(), CommandEntry, command)
	if err != nil {
		return 0, fmt.Errorf("failed to append to log: %v", err)
	}
	
	// Replicate to peers
	for peerID := range n.Config.Peers {
		if peerID == n.ID {
			continue // Skip self
		}
		
		go n.sendAppendEntries(peerID, false)
	}
	
	return index, nil
}

// GetState returns the current state and term
func (n *Node) GetState() (NodeState, uint64) {
	return n.VolatileState.GetState(), n.RaftState.GetCurrentTerm()
}

// IsLeader returns true if this node is the leader
func (n *Node) IsLeader() bool {
	return n.VolatileState.GetState() == Leader
}

// GetLeaderID returns the ID of the current leader (empty if unknown)
func (n *Node) GetLeaderID() string {
	// TODO: Track the current leader ID
	return ""
}

// startElection sends RequestVote RPCs to all peers
func (n *Node) startElection() {
	n.Logger.Info("Starting election for term %d", n.RaftState.GetCurrentTerm())

	// Prepare RequestVote arguments
	lastLogEntry := n.Log.GetLastEntry()
	args := &protocol.RequestVoteArgs{
		Term:         n.RaftState.GetCurrentTerm(),
		CandidateID:  n.ID,
		LastLogIndex: lastLogEntry.Index,
		LastLogTerm:  lastLogEntry.Term,
	}

	// Send RequestVote RPCs to all peers
	for peerID := range n.Config.Peers {
		if peerID == n.ID {
			continue // Skip self
		}

		go n.sendRequestVote(peerID, args)
	}
}

// sendRequestVote sends a RequestVote RPC to a peer
func (n *Node) sendRequestVote(peerID string, args *protocol.RequestVoteArgs) {
	// Serialize arguments
	data, err := protocol.SerializeRequestVoteArgs(args)
	if err != nil {
		n.Logger.Error("Failed to serialize RequestVote args: %v", err)
		return
	}

	// Create RPC message
	msg := &protocol.RPCMessage{
		Type: protocol.RequestVoteRequest,
		From: n.ID,
		To:   peerID,
		Data: data,
	}

	// Send message
	if err := n.Transport.SendMessage(msg); err != nil {
		n.Logger.Error("Failed to send RequestVote to %s: %v", peerID, err)
	} else {
		n.Logger.Debug("Sent RequestVote to %s for term %d", peerID, args.Term)
	}
}