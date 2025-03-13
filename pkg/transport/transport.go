package transport

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/sauravfouzdar/raft/pkg/logger"
	"github.com/sauravfouzdar/raft/pkg/protocol"
)

// TCPTransport implements the Transport interface using TCP
type TCPTransport struct {
	
	nodeID string
	
	addr string

	// peers is a map of peer node IDs to their addresses
	peers map[string]string

	// listener is the TCP listener
	listener net.Listener

	// msgCh is the channel of incoming messages
	msgCh chan *protocol.RPCMessage

	// logger is used for logging
	logger *logger.Logger

	// closed indicates if the transport has been closed
	closed bool

	// mu protects access to the transport
	mu sync.RWMutex

	// wg is used to wait for all connections to close
	wg sync.WaitGroup
}

// NewTCPTransport creates a new TCPTransport
func NewTCPTransport(nodeID, addr string, peers map[string]string, logger *logger.Logger) *TCPTransport {
	return &TCPTransport{
		nodeID:  nodeID,
		addr:    addr,
		peers:   peers,
		msgCh:   make(chan *protocol.RPCMessage, 1000),
		logger:  logger,
		closed:  false,
	}
}

// Start starts the transport
func (t *TCPTransport) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return fmt.Errorf("transport already closed")
	}

	// Start TCP listener
	listener, err := net.Listen("tcp", t.addr)
	if err != nil {
		return fmt.Errorf("failed to start listener: %v", err)
	}
	t.listener = listener

	t.logger.Info("Transport started on %s", t.addr)

	// Accept connections in a goroutine
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.acceptConnections()
	}()

	return nil
}

// acceptConnections accepts new connections
func (t *TCPTransport) acceptConnections() {
	for {
		// Check if transport is closed
		t.mu.RLock()
		closed := t.closed
		listener := t.listener
		t.mu.RUnlock()

		if closed {
			return
		}

		conn, err := listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				t.logger.Warn("Temporary error accepting connection: %v", err)
				continue
			}

			// Check again if transport is closed
			t.mu.RLock()
			closed := t.closed
			t.mu.RUnlock()

			if closed {
				return
			}

			t.logger.Error("Error accepting connection: %v", err)
			return
		}

		// Handle connection in a goroutine
		t.wg.Add(1)
		go func() {
			defer t.wg.Done()
			t.handleConnection(conn)
		}()
	}
}

// handleConnection handles a connection
func (t *TCPTransport) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Decode message
	var msg protocol.RPCMessage
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&msg); err != nil {
		t.logger.Error("Error decoding message: %v", err)
		return
	}

	// Verify message is for this node
	if msg.To != t.nodeID {
		t.logger.Warn("Received message for %s, but we are %s", msg.To, t.nodeID)
		return
	}

	// Send message to channel
	select {
	case t.msgCh <- &msg:
		// Message sent successfully
	default:
		t.logger.Warn("Message channel full, dropping message")
	}
}

// Stop stops the transport
func (t *TCPTransport) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true

	// Close listener
	if t.listener != nil {
		t.listener.Close()
	}

	// Wait for all connections to close
	t.wg.Wait()

	t.logger.Info("Transport stopped")
	return nil
}

// SendMessage sends an RPC message to another node
func (t *TCPTransport) SendMessage(msg *protocol.RPCMessage) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed {
		return fmt.Errorf("transport closed")
	}

	// Get peer address
	addr, ok := t.peers[msg.To]
	if !ok {
		return fmt.Errorf("unknown peer: %s", msg.To)
	}

	// Set from field
	msg.From = t.nodeID

	// Connect to peer
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", addr, err)
	}
	defer conn.Close()

	// Encode and send message
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(msg); err != nil {
		return fmt.Errorf("failed to encode message: %v", err)
	}

	return nil
}

// Messages returns a channel of incoming messages
func (t *TCPTransport) Messages() <-chan *protocol.RPCMessage {
	return t.msgCh
}