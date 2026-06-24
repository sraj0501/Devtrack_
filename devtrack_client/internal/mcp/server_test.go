package mcp

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func makeServer() *Server {
	s := New("test-version")
	return s
}

func sendAndReceive(t *testing.T, s *Server, input string) jsonRPCResponse {
	t.Helper()
	pr, pw := io.Pipe()
	outPR, outPW := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		s.run(ctx, pr, outPW)
		outPW.Close()
	}()

	_, _ = io.WriteString(pw, input+"\n")
	pw.Close()

	var resp jsonRPCResponse
	dec := json.NewDecoder(outPR)
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestServerInitialize(t *testing.T) {
	s := makeServer()
	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"0.1"}}}`
	resp := sendAndReceive(t, s, req)

	if resp.Error != nil {
		t.Fatalf("expected no error, got: %v", resp.Error)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", resp.Result)
	}
	info, _ := result["serverInfo"].(map[string]interface{})
	if info["name"] != "devtrack" {
		t.Errorf("expected serverInfo.name=devtrack, got %v", info["name"])
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocolVersion=2024-11-05, got %v", result["protocolVersion"])
	}
}

func TestServerToolsCall_Unknown(t *testing.T) {
	s := makeServer()
	req := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"nonexistent","arguments":{}}}`
	resp := sendAndReceive(t, s, req)

	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected error code -32602, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "nonexistent") {
		t.Errorf("expected error message to mention tool name, got: %s", resp.Error.Message)
	}
}

func TestServerShutdown(t *testing.T) {
	s := makeServer()
	pr, pw := io.Pipe()
	outPR, outPW := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.run(ctx, pr, outPW)
		outPW.Close()
		close(done)
	}()

	_, _ = io.WriteString(pw, `{"jsonrpc":"2.0","id":3,"method":"shutdown"}`+"\n")
	pw.Close()

	// Drain the response
	io.ReadAll(outPR)

	select {
	case <-done:
		// ok — server exited
	case <-time.After(2 * time.Second):
		t.Fatal("server did not exit after shutdown within 2 seconds")
	}
}

func TestServerPing(t *testing.T) {
	s := makeServer()
	req := `{"jsonrpc":"2.0","id":4,"method":"ping"}`
	resp := sendAndReceive(t, s, req)

	if resp.Error != nil {
		t.Fatalf("expected no error for ping, got: %v", resp.Error)
	}
}
