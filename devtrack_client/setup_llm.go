package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ollamaTagsResponse struct {
	Models []struct {
		Name    string `json:"name"`
		Model   string `json:"model"`
		Details struct {
			Family   string   `json:"family"`
			Families []string `json:"families"`
		} `json:"details"`
	} `json:"models"`
}

type setupCloudKeys struct {
	OpenAI    string
	Anthropic string
}

func setupHTTPClient() *http.Client {
	return &http.Client{Timeout: time.Duration(GetHTTPTimeoutShort()) * time.Second}
}

// detectUsableOllamaModel returns an installed generation model. Embedding-only
// models cannot produce commit messages or reports, so they are ignored.
func detectUsableOllamaModel(host string, client *http.Client) (string, error) {
	base, err := normalizeSetupOllamaHost(host)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequest(http.MethodGet, base+"/api/tags", nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama tags returned %s", response.Status)
	}
	var tags ollamaTagsResponse
	if err := json.NewDecoder(response.Body).Decode(&tags); err != nil {
		return "", fmt.Errorf("decode ollama tags: %w", err)
	}
	for _, candidate := range tags.Models {
		name := strings.TrimSpace(candidate.Name)
		if name == "" {
			name = strings.TrimSpace(candidate.Model)
		}
		if name != "" && !isEmbeddingOnlyModel(name, candidate.Details.Family, candidate.Details.Families) {
			return name, nil
		}
	}
	return "", nil
}

func normalizeSetupOllamaHost(raw string) (string, error) {
	host := strings.TrimRight(strings.TrimSpace(raw), "/")
	if host == "" {
		return "", fmt.Errorf("Ollama host is empty")
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	parsed, err := url.Parse(host)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("invalid Ollama host %q", raw)
	}
	hostname := parsed.Hostname()
	if hostname == "0.0.0.0" || hostname == "::" {
		hostname = "localhost"
	}
	port := parsed.Port()
	if port == "" {
		port = "11434"
	}
	parsed.Host = net.JoinHostPort(hostname, port)
	parsed.RawQuery, parsed.Fragment = "", ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func isEmbeddingOnlyModel(name, family string, families []string) bool {
	combined := strings.ToLower(name + " " + family + " " + strings.Join(families, " "))
	for _, marker := range []string{"embed", "nomic", "mxbai", "bge", "bert"} {
		if strings.Contains(combined, marker) {
			return true
		}
	}
	return false
}

func configureOllamaSetup(reader *bufio.Reader, cfg *SetupConfig, detectedHost, detectedModel string) {
	cfg.LLMProvider = "ollama"
	fmt.Printf("Ollama host [%s]: ", detectedHost)
	host := readLine(reader)
	if host == "" {
		host = detectedHost
	}
	cfg.OllamaHost = host

	model := ""
	if host == detectedHost {
		model = detectedModel
	} else {
		model, _ = detectUsableOllamaModel(host, setupHTTPClient())
	}
	if model != "" {
		cfg.OllamaModel = model
		cfg.OllamaModelReady = true
		fmt.Printf("  ✓ Using installed Ollama model %s (no download needed)\n", model)
		return
	}

	fmt.Print("Ollama model to download [llama3.2]: ")
	model = readLine(reader)
	if model == "" {
		model = "llama3.2"
	}
	cfg.OllamaModel = model
	offerCloudFastLane(reader, cfg, setupCloudKeys{
		OpenAI: GetOpenAIAPIKeyOptional(), Anthropic: GetAnthropicAPIKeyOptional(),
	})
}

func offerCloudFastLane(reader *bufio.Reader, cfg *SetupConfig, keys setupCloudKeys) {
	providers := make([]string, 0, 2)
	if keys.OpenAI != "" {
		providers = append(providers, "OpenAI")
	}
	if keys.Anthropic != "" {
		providers = append(providers, "Anthropic")
	}
	if len(providers) == 0 {
		return
	}
	fmt.Printf("  %s API key detected.\n", strings.Join(providers, " and "))
	fmt.Println("  Cloud fallback sends prompt text to that provider; Ollama remains primary.")
	fmt.Print("  Use the detected key while the local model downloads? [Y/n]: ")
	answer := strings.ToLower(readLine(reader))
	if answer == "n" || answer == "no" {
		fmt.Println("  Keeping setup local-only.")
		return
	}
	cfg.OpenAIKey = keys.OpenAI
	cfg.AnthropicKey = keys.Anthropic
	fmt.Println("  ✓ Cloud fast lane enabled; DevTrack automatically returns to Ollama when ready.")
}
