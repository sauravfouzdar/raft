package protocol

import (
	"encoding/json"
)

// LogEntryType defines the type of a log entry
type LogEntryType int

const (
	// CommandEntry is a log entry that contains a command for the state machine
	CommandEntry LogEntryType = iota
	// ConfigurationEntry is a log entry that contains a configuration change
	ConfigurationEntry
	// NoOpEntry is a log entry that contains no operation
	NoOpEntry
)

// LogEntry represents an entry in the Raft log
type LogEntry struct {
	// Index is the position of the entry in the log
	Index uint64

	// Term is the term when the entry was received by the leader
	Term uint64

	// Type is the type of the log entry
	Type LogEntryType

	// Command is the command to be applied to the state machine
	Command []byte
}

// RPCType defines the type of RPC message
type RPCType int

const (
	// AppendEntriesRequest is sent by the leader to replicate log entries
	AppendEntriesRequest RPCType = iota
	// AppendEntriesResponse is the response to an AppendEntriesRequest
	AppendEntriesResponse
	// RequestVoteRequest is sent by candidates to gather votes
	RequestVoteRequest
	// RequestVoteResponse is the response to a RequestVoteRequest
	RequestVoteResponse
)

// RPCMessage represents a message sent between Raft nodes
type RPCMessage struct {
	// Type is the type of RPC message
	Type RPCType

	// From is the ID of the sending node
	From string

	// To is the ID of the target node
	To string

	// Data is the serialized RPC data
	Data []byte
}

// AppendEntriesArgs represents the arguments for an AppendEntries RPC
type AppendEntriesArgs struct {
	// Term is the leader's term
	Term uint64

	// LeaderID is the ID of the leader
	LeaderID string

	// PrevLogIndex is the index of the log entry immediately preceding new ones
	PrevLogIndex uint64

	// PrevLogTerm is the term of the PrevLogIndex entry
	PrevLogTerm uint64

	// Entries is the log entries to store (empty for heartbeat)
	Entries []LogEntry

	// LeaderCommit is the leader's commitIndex
	LeaderCommit uint64
}

// AppendEntriesReply represents the result of an AppendEntries RPC
type AppendEntriesReply struct {
	// Term is the current term, for leader to update itself
	Term uint64

	// Success is true if follower contained entry matching PrevLogIndex and PrevLogTerm
	Success bool

	// NextIndex is the next index to try if success is false (optimization)
	NextIndex uint64
}

// RequestVoteArgs represents the arguments for a RequestVote RPC
type RequestVoteArgs struct {
	// Term is the candidate's term
	Term uint64

	// CandidateID is the ID of the candidate
	CandidateID string

	// LastLogIndex is the index of the candidate's last log entry
	LastLogIndex uint64

	// LastLogTerm is the term of the candidate's last log entry
	LastLogTerm uint64
}

// RequestVoteReply represents the result of a RequestVote RPC
type RequestVoteReply struct {
	// Term is the current term, for candidate to update itself
	Term uint64

	// VoteGranted is true if the candidate received vote
	VoteGranted bool
}

// Transport defines the interface for network communication between Raft nodes
type Transport interface {
	// Start starts the transport
	Start() error

	// Stop stops the transport
	Stop() error

	// SendMessage sends an RPC message to another node
	SendMessage(msg *RPCMessage) error

	// Messages returns a channel of incoming messages
	Messages() <-chan *RPCMessage
}

// SerializeAppendEntriesArgs serializes AppendEntriesArgs to JSON
func SerializeAppendEntriesArgs(args *AppendEntriesArgs) ([]byte, error) {
	return json.Marshal(args)
}

// DeserializeAppendEntriesArgs deserializes AppendEntriesArgs from JSON
func DeserializeAppendEntriesArgs(data []byte) (*AppendEntriesArgs, error) {
	var args AppendEntriesArgs
	err := json.Unmarshal(data, &args)
	return &args, err
}

// SerializeAppendEntriesReply serializes AppendEntriesReply to JSON
func SerializeAppendEntriesReply(reply *AppendEntriesReply) ([]byte, error) {
	return json.Marshal(reply)
}

// DeserializeAppendEntriesReply deserializes AppendEntriesReply from JSON
func DeserializeAppendEntriesReply(data []byte) (*AppendEntriesReply, error) {
	var reply AppendEntriesReply
	err := json.Unmarshal(data, &reply)
	return &reply, err
}

// SerializeRequestVoteArgs serializes RequestVoteArgs to JSON
func SerializeRequestVoteArgs(args *RequestVoteArgs) ([]byte, error) {
	return json.Marshal(args)
}

// DeserializeRequestVoteArgs deserializes RequestVoteArgs from JSON
func DeserializeRequestVoteArgs(data []byte) (*RequestVoteArgs, error) {
	var args RequestVoteArgs
	err := json.Unmarshal(data, &args)
	return &args, err
}

// SerializeRequestVoteReply serializes RequestVoteReply to JSON
func SerializeRequestVoteReply(reply *RequestVoteReply) ([]byte, error) {
	return json.Marshal(reply)
}

// DeserializeRequestVoteReply deserializes RequestVoteReply from JSON
func DeserializeRequestVoteReply(data []byte) (*RequestVoteReply, error) {
	var reply RequestVoteReply
	err := json.Unmarshal(data, &reply)
	return &reply, err
}

// SerializeRPCMessage serializes an RPCMessage to JSON
func SerializeRPCMessage(msg *RPCMessage) ([]byte, error) {
	return json.Marshal(msg)
}

// DeserializeRPCMessage deserializes an RPCMessage from JSON
func DeserializeRPCMessage(data []byte) (*RPCMessage, error) {
	var msg RPCMessage
	err := json.Unmarshal(data, &msg)
	return &msg, err
}

// SerializeEntry serializes a log entry to JSON
func SerializeEntry(entry LogEntry) ([]byte, error) {
	return json.Marshal(entry)
}

// DeserializeEntry deserializes a log entry from JSON
func DeserializeEntry(data []byte) (LogEntry, error) {
	var entry LogEntry
	err := json.Unmarshal(data, &entry)
	return entry, err
}