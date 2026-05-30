package match

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Embedder turns text into a vector for semantic similarity.
type Embedder interface {
	Embed(text string) ([]float64, error)
}

// NewOllamaEmbedder returns an Ollama-backed embedder, or nil when semantic
// matching is not configured. It is opt-in: PM_MATCH_EMBED_MODEL must name an
// embedding model (e.g. "nomic-embed-text") and OLLAMA_HOST must be set. When
// nil, callers fall back to offline fuzzy ranking — preserving offline-first.
func NewOllamaEmbedder() Embedder {
	model := strings.TrimSpace(os.Getenv("PM_MATCH_EMBED_MODEL"))
	if model == "" {
		return nil
	}
	host := strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
	if host == "" {
		return nil
	}
	return &ollamaEmbedder{
		host:  strings.TrimRight(host, "/"),
		model: model,
		http:  &http.Client{Timeout: 10 * time.Second},
	}
}

type ollamaEmbedder struct {
	host  string
	model string
	http  *http.Client
}

func (e *ollamaEmbedder) Embed(text string) ([]float64, error) {
	payload, err := json.Marshal(map[string]any{"model": e.model, "input": text})
	if err != nil {
		return nil, err
	}
	resp, err := e.http.Post(e.host+"/api/embed", "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ollama embed error %d: %s", resp.StatusCode, string(body))
	}

	// Ollama /api/embed returns {"embeddings":[[...]]}; tolerate the older
	// single-vector {"embedding":[...]} shape too.
	var out struct {
		Embeddings [][]float64 `json:"embeddings"`
		Embedding  []float64   `json:"embedding"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("ollama embed decode: %w", err)
	}
	if len(out.Embeddings) > 0 {
		return out.Embeddings[0], nil
	}
	if len(out.Embedding) > 0 {
		return out.Embedding, nil
	}
	return nil, fmt.Errorf("ollama embed: empty response")
}
