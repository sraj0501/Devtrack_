package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// MessageType defines the type of IPC message
type MessageType string

const (
	// Message types for Go -> Python
	MsgTypeCommitTrigger MessageType = "commit_trigger"
	MsgTypeTimerTrigger  MessageType = "timer_trigger"
	MsgTypeStatusQuery   MessageType = "status_query"
	MsgTypeShutdown      MessageType = "shutdown"
	MsgTypeConfigUpdate  MessageType = "config_update"

	// Message types for Python -> Go
	MsgTypeResponse      MessageType = "response"
	MsgTypeTaskUpdate    MessageType = "task_update"
	MsgTypeError         MessageType = "error"
	MsgTypeAck           MessageType = "ack"
	MsgTypePromptRequest MessageType = "prompt_request"
)

// IPCMessage represents a message sent between Go and Python
type IPCMessage struct {
	Type      MessageType            `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	ID        string                 `json:"id"`
	Data      map[string]interface{} `json:"data"`
	Error     string                 `json:"error,omitempty"`
}

// CommitTriggerData contains information about a Git commit
type CommitTriggerData struct {
	RepoPath      string   `json:"repo_path"`
	CommitHash    string   `json:"commit_hash"`
	CommitMessage string   `json:"commit_message"`
	Author        string   `json:"author"`
	Timestamp     string   `json:"timestamp"`
	FilesChanged  []string `json:"files_changed"`
	Branch        string   `json:"branch"`
}

// TimerTriggerData contains information about a scheduled trigger
type TimerTriggerData struct {
	Timestamp    string `json:"timestamp"`
	IntervalMins int    `json:"interval_mins"`
	TriggerCount int    `json:"trigger_count"`
}

// TaskUpdateData contains information about a task update
type TaskUpdateData struct {
	Project     string `json:"project"`
	TicketID    string `json:"ticket_id"`
	Description string `json:"description"`
	Status      string `json:"status"`
	TimeSpent   string `json:"time_spent"`
	Synced      bool   `json:"synced"`
}

// IPCServer manages IPC communication
type IPCServer struct {
	socketPath string
	listener   net.Listener
	clients    map[string]net.Conn
	mu         sync.RWMutex
	running    bool
	handlers   map[MessageType]func(msg IPCMessage) error
}

// IPCClient manages client-side IPC communication
type IPCClient struct {
	socketPath string
	conn       net.Conn
	mu         sync.Mutex
	connected  bool
}

// NewIPCServer creates a new IPC server
func NewIPCServer() (*IPCServer, error) {
	socketPath, err := getSocketPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get socket path: %w", err)
	}

	server := &IPCServer{
		socketPath: socketPath,
		clients:    make(map[string]net.Conn),
		handlers:   make(map[MessageType]func(msg IPCMessage) error),
	}

	return server, nil
}

// Start begins listening for IPC connections
func (s *IPCServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("IPC server already running")
	}

	log.Printf("Attempting to start IPC server on: %s", s.socketPath)
	var err error
	s.listener, err = net.Listen("tcp", s.socketPath)
	if err != nil {
		log.Printf("ERROR: Failed to start TCP listener: %v", err)
		return fmt.Errorf("failed to start IPC listener: %w", err)
	}

	log.Printf("✓ IPC server bound to: %s", s.socketPath)

	s.running = true
	log.Printf("IPC server listening on %s", s.socketPath)

	// Start accepting connections in a goroutine
	go s.acceptConnections()

	// Start socket keepalive monitoring
	go s.monitorSocket()

	return nil
}

// monitorSocket periodically checks if the listener is still accepting
func (s *IPCServer) monitorSocket() {
	// For TCP sockets, we don't need to monitor file existence
	// The listener will be alive as long as the process is alive
	log.Printf("Socket monitor: IPC server on %s", s.socketPath)
}

// Stop closes the IPC server
func (s *IPCServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	log.Println("Stopping IPC server...")
	s.running = false

	// Close all client connections
	for id, conn := range s.clients {
		conn.Close()
		delete(s.clients, id)
	}

	// Close listener
	if s.listener != nil {
		log.Println("Closing IPC listener...")
		s.listener.Close()
	}

	log.Println("IPC server stopped")
	return nil
}

// acceptConnections handles incoming client connections
func (s *IPCServer) acceptConnections() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				log.Printf("Error accepting connection: %v", err)
			}
			continue
		}

		clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())
		s.mu.Lock()
		s.clients[clientID] = conn
		s.mu.Unlock()

		log.Printf("New IPC client connected: %s", clientID)

		// Handle client in a goroutine
		go s.handleClient(clientID, conn)
	}
}

// handleClient processes messages from a connected client
func (s *IPCServer) handleClient(clientID string, conn net.Conn) {
	defer func() {
		conn.Close()
		s.mu.Lock()
		delete(s.clients, clientID)
		s.mu.Unlock()
		log.Printf("IPC client disconnected: %s", clientID)
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()

		var msg IPCMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			log.Printf("Error parsing IPC message: %v", err)
			continue
		}

		// Handle message
		if handler, ok := s.handlers[msg.Type]; ok {
			if err := handler(msg); err != nil {
				log.Printf("Error handling message type %s: %v", msg.Type, err)
			}
		} else {
			log.Printf("No handler for message type: %s", msg.Type)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from client %s: %v", clientID, err)
	}
}

// RegisterHandler registers a handler function for a message type
func (s *IPCServer) RegisterHandler(msgType MessageType, handler func(msg IPCMessage) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[msgType] = handler
}

// SendMessage sends a message to all connected clients
func (s *IPCServer) SendMessage(msg IPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.clients) == 0 {
		// No clients connected - this is expected initially
		log.Printf("No IPC clients connected, message queued or dropped: %s", msg.Type)
		return nil
	}

	// Add newline delimiter
	data = append(data, '\n')

	for id, conn := range s.clients {
		if _, err := conn.Write(data); err != nil {
			log.Printf("Error sending message to client %s: %v", id, err)
		}
	}

	return nil
}

// NewIPCClient creates a new IPC client
func NewIPCClient() (*IPCClient, error) {
	socketPath, err := getSocketPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get socket path: %w", err)
	}

	return &IPCClient{
		socketPath: socketPath,
		connected:  false,
	}, nil
}

// Connect establishes connection to the IPC server
func (c *IPCClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	// Try to connect with timeout from config
	timeoutSecs := GetIPCConnectTimeoutSecs()
	conn, err := net.DialTimeout("unix", c.socketPath, time.Duration(timeoutSecs)*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to IPC server: %w", err)
	}

	c.conn = conn
	c.connected = true
	log.Println("Connected to IPC server")

	return nil
}

// Disconnect closes the connection
func (c *IPCClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	if c.conn != nil {
		c.conn.Close()
	}

	c.connected = false
	log.Println("Disconnected from IPC server")

	return nil
}

// SendMessage sends a message to the server
func (c *IPCClient) SendMessage(msg IPCMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return fmt.Errorf("not connected to IPC server")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Add newline delimiter
	data = append(data, '\n')

	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// ReceiveMessage receives a message from the server
func (c *IPCClient) ReceiveMessage() (*IPCMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to IPC server")
	}

	reader := bufio.NewReader(c.conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("connection closed")
		}
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	var msg IPCMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}

// StartListening starts listening for messages in a goroutine
func (c *IPCClient) StartListening(handler func(msg IPCMessage) error) {
	go func() {
		for c.connected {
			msg, err := c.ReceiveMessage()
			if err != nil {
				if c.connected {
					log.Printf("Error receiving message: %v", err)
				}
				break
			}

			if err := handler(*msg); err != nil {
				log.Printf("Error handling message: %v", err)
			}
		}
	}()
}

// getSocketPath returns the platform-specific socket path
func getSocketPath() (string, error) {
	// Use TCP socket instead of Unix socket for better container compatibility
	// Unix sockets can have issues with docker exec -d due to file descriptor handling
	return GetIPCAddress(), nil  // Get from environment config
}

// CreateCommitTriggerMessage creates a commit trigger message
func CreateCommitTriggerMessage(data CommitTriggerData) IPCMessage {
	return IPCMessage{
		Type:      MsgTypeCommitTrigger,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("commit_%d", time.Now().UnixNano()),
		Data: map[string]interface{}{
			"repo_path":      data.RepoPath,
			"commit_hash":    data.CommitHash,
			"commit_message": data.CommitMessage,
			"author":         data.Author,
			"timestamp":      data.Timestamp,
			"files_changed":  data.FilesChanged,
			"branch":         data.Branch,
		},
	}
}

// CreateTimerTriggerMessage creates a timer trigger message
func CreateTimerTriggerMessage(data TimerTriggerData) IPCMessage {
	return IPCMessage{
		Type:      MsgTypeTimerTrigger,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("timer_%d", time.Now().UnixNano()),
		Data: map[string]interface{}{
			"timestamp":     data.Timestamp,
			"interval_mins": data.IntervalMins,
			"trigger_count": data.TriggerCount,
		},
	}
}

// CreateTaskUpdateMessage creates a task update message
func CreateTaskUpdateMessage(data TaskUpdateData) IPCMessage {
	return IPCMessage{
		Type:      MsgTypeTaskUpdate,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("task_%d", time.Now().UnixNano()),
		Data: map[string]interface{}{
			"project":     data.Project,
			"ticket_id":   data.TicketID,
			"description": data.Description,
			"status":      data.Status,
			"time_spent":  data.TimeSpent,
			"synced":      data.Synced,
		},
	}
}

// CreateResponseMessage creates a response message
func CreateResponseMessage(requestID string, data map[string]interface{}) IPCMessage {
	return IPCMessage{
		Type:      MsgTypeResponse,
		Timestamp: time.Now(),
		ID:        requestID,
		Data:      data,
	}
}

// CreateErrorMessage creates an error message
func CreateErrorMessage(requestID string, errorMsg string) IPCMessage {
	return IPCMessage{
		Type:      MsgTypeError,
		Timestamp: time.Now(),
		ID:        requestID,
		Error:     errorMsg,
		Data:      make(map[string]interface{}),
	}
}
