package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"sync"
)

const latestHandshakeProtocolVersion = "2025-11-25"

var supportedHandshakeProtocolVersions = map[string]bool{
	"2024-11-05": true,
	"2025-03-26": true,
	"2025-06-18": true,
	"2025-11-25": true,
}

// jsonRPCRequest is an inbound JSON-RPC 2.0 message from the MCP client.
type jsonRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

// jsonRPCResponse is an outbound JSON-RPC 2.0 message to the MCP client.
type jsonRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool is a registered MCP tool: its declaration and its handler.
type Tool struct {
	Name        string
	Title       string
	Description string
	InputSchema map[string]any // JSON Schema for the tool's input object
	Annotations map[string]any // MCP behavioral hints for clients
	Handler     func(ctx context.Context, args map[string]any) (any, error)
}

// Server is the DevTrack MCP server. It handles JSON-RPC 2.0 over stdio.
type Server struct {
	mu      sync.Mutex
	tools   map[string]Tool
	version string
}

// New creates a new Server with no tools registered.
func New(version string) *Server {
	return &Server{
		tools:   make(map[string]Tool),
		version: version,
	}
}

// Register adds a tool to the server. Call before Start().
// Panics if a tool with the same name is already registered.
func (s *Server) Register(t Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tools[t.Name]; exists {
		panic(fmt.Sprintf("mcp: tool %q already registered", t.Name))
	}
	s.tools[t.Name] = t
}

// Start runs the JSON-RPC 2.0 message loop on os.Stdin / os.Stdout.
// Blocks until the client sends "shutdown" or ctx is cancelled.
func (s *Server) Start(ctx context.Context) {
	s.run(ctx, os.Stdin, os.Stdout)
}

// RunOn runs the JSON-RPC 2.0 message loop on the provided reader/writer.
// Exported for use by devtrack mcp test (in-process smoke test).
// Blocks until the client sends "shutdown" or ctx is cancelled.
func (s *Server) RunOn(ctx context.Context, in io.Reader, out io.Writer) {
	s.run(ctx, in, out)
}

// run is the testable implementation of Start.
func (s *Server) run(ctx context.Context, in io.Reader, out io.Writer) {
	reader := bufio.NewReader(in)
	for {
		// Check context before blocking read
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Printf("mcp: read error: %v", err)
			return
		}

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("mcp: parse error: %v (line: %q)", err, line)
			s.writeResponse(out, jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &jsonRPCError{Code: -32700, Message: "Parse error"},
			})
			continue
		}

		// JSON-RPC notifications never receive a response. This covers the MCP
		// initialized notification and safely ignores future client notifications.
		if req.ID == nil {
			continue
		}
		if req.Method == "shutdown" {
			s.writeResponse(out, jsonRPCResponse{
				JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage("null"),
			})
			return
		}

		resp := s.handle(ctx, req)
		s.writeResponse(out, resp)
	}
}

func (s *Server) handle(ctx context.Context, req jsonRPCRequest) jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "ping":
		return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	default:
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32601, Message: "Method not found: " + req.Method},
		}
	}
}

func (s *Server) handleInitialize(req jsonRPCRequest) jsonRPCResponse {
	protocolVersion := latestHandshakeProtocolVersion
	if requested, _ := req.Params["protocolVersion"].(string); supportedHandshakeProtocolVersions[requested] {
		protocolVersion = requested
	}
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"protocolVersion": protocolVersion,
			"serverInfo": map[string]any{
				"name":    "devtrack",
				"version": s.version,
			},
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
		},
	}
}

func (s *Server) handleToolsList(req jsonRPCRequest) jsonRPCResponse {
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"tools": s.toolList(),
		},
	}
}

func (s *Server) toolList() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.tools))
	for name := range s.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	list := make([]map[string]any, 0, len(names))
	for _, name := range names {
		t := s.tools[name]
		entry := map[string]any{
			"name":        t.Name,
			"description": t.Description,
		}
		if t.Title != "" {
			entry["title"] = t.Title
		}
		if t.Annotations != nil {
			entry["annotations"] = t.Annotations
		}
		if t.InputSchema != nil {
			entry["inputSchema"] = t.InputSchema
		} else {
			entry["inputSchema"] = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		list = append(list, entry)
	}
	return list
}

func (s *Server) handleToolsCall(ctx context.Context, req jsonRPCRequest) jsonRPCResponse {
	params := req.Params
	if params == nil {
		params = map[string]any{}
	}

	nameRaw, _ := params["name"].(string)
	if nameRaw == "" {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32602, Message: "Missing tool name"},
		}
	}

	s.mu.Lock()
	tool, ok := s.tools[nameRaw]
	s.mu.Unlock()

	if !ok {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32602, Message: "Unknown tool: " + nameRaw},
		}
	}

	args, _ := params["arguments"].(map[string]any)
	if args == nil {
		args = map[string]any{}
	}

	result, err := tool.Handler(ctx, args)
	if err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32603, Message: err.Error()},
		}
	}

	// MCP content format: wrap result as a text content block
	resultJSON, _ := json.Marshal(result)
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": string(resultJSON)},
			},
		},
	}
}

func (s *Server) writeResponse(out io.Writer, resp jsonRPCResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("mcp: marshal error: %v", err)
		return
	}
	_, _ = fmt.Fprintf(out, "%s\n", data)
}
