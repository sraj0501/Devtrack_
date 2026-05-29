package infra

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
)

// infraState holds the ports chosen for DevTrack-managed Docker containers.
// Written once on first provision; read on every restart to reuse the same ports.
type infraState struct {
	MongoPort string `json:"mongo_port"`
	RedisPort string `json:"redis_port"`
}

func infraStatePath() string {
	home, err := config.DevtrackDataHome()
	if err != nil {
		// Fallback to ~/.devtrack so we always have somewhere to write.
		if h, e := os.UserHomeDir(); e == nil {
			return filepath.Join(h, ".devtrack", "infra_state.json")
		}
		return ""
	}
	return filepath.Join(home, "data", "infra_state.json")
}

func readInfraState() infraState {
	path := infraStatePath()
	if path == "" {
		return infraState{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return infraState{}
	}
	var s infraState
	_ = json.Unmarshal(data, &s)
	return s
}

func writeInfraState(s infraState) {
	path := infraStatePath()
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0644)
}

const (
	mongoContainerName = "devtrack-mongo"
	redisContainerName = "devtrack-redis"
	mongoImage         = "mongo:7.0"
	redisImage         = "redis:7.2-alpine"
)

// EnsureLocalInfra verifies MongoDB and Redis are reachable before the daemon starts.
// If either is missing it tries to auto-start Docker containers (random host ports).
// If Docker is also absent it enters an interactive loop:
//   - option 1: wait here until the user installs/starts Docker, then auto-provision
//   - option 2: enter connection strings manually
//   - option 3: exit (daemon will not start)
//
// Returns a non-nil error only when the user chooses to exit without a working DB.
// Called from handleStart() in the parent process only (not the daemon child).
func EnsureLocalInfra() error {
	mongoHost, mongoPort, mongoUser, mongoPass, mongoDB := resolveMongoConfig()
	redisHost, redisPort, redisPass := resolveRedisConfig()

	mongoOK := tcpPing(mongoHost, mongoPort)
	redisOK := tcpPing(redisHost, redisPort)

	if mongoOK && redisOK {
		return nil
	}

	printMissingServices(mongoOK, redisOK, mongoHost, mongoPort, redisHost, redisPort)

	if dockerAvailable() {
		return provisionContainers(mongoOK, redisOK, mongoUser, mongoPass, mongoDB, redisPass)
	}

	// Docker not found — enter interactive resolution loop.
	return infraSetupLoop(mongoOK, redisOK, mongoUser, mongoPass, mongoDB, redisPass)
}

// provisionContainers starts whichever of MongoDB / Redis are missing as Docker containers.
// Preferred ports are read from the infra state file so the same ports are reused across
// restarts. New random ports are only chosen when no preference is stored or the stored
// port is already bound by another process.
func provisionContainers(mongoOK bool, redisOK bool, mongoUser, mongoPass, mongoDB, redisPass string) error {
	state := readInfraState()

	if !mongoOK {
		port, err := ensureMongoRunning(mongoUser, mongoPass, state.MongoPort)
		if err != nil {
			return fmt.Errorf("failed to start MongoDB: %w", err)
		}
		uri := fmt.Sprintf("mongodb://%s:%s@localhost:%s/%s?authSource=admin",
			mongoUser, mongoPass, port, mongoDB)
		os.Setenv("MONGODB_URI", uri)
		updateEnvFile("MONGODB_URI", uri)
		state.MongoPort = port
		fmt.Printf("   ✓ MongoDB ready → %s\n", uri)
	}

	if !redisOK {
		port, err := ensureRedisRunning(redisPass, state.RedisPort)
		if err != nil {
			return fmt.Errorf("failed to start Redis: %w", err)
		}
		url := fmt.Sprintf("redis://:%s@localhost:%s/0", redisPass, port)
		os.Setenv("REDIS_URL", url)
		updateEnvFile("REDIS_URL", url)
		state.RedisPort = port
		fmt.Printf("   ✓ Redis ready → %s\n", url)
	}

	writeInfraState(state)
	return nil
}

// ensureMongoRunning ensures the MongoDB container is running and returns the host port it listens on.
//
// Port selection priority (once the container needs to be created fresh):
//  1. preferredPort from the infra state file — reuses the installation-time port when possible
//  2. Fresh random port — only when the preferred port is already bound by another process
//
// If the container already exists (running or stopped), its actual mapped port is always used
// regardless of preferredPort, so the state file stays in sync.
func ensureMongoRunning(user, pass, preferredPort string) (string, error) {
	status := containerStatus(mongoContainerName)

	if status == "running" {
		// Container is already up — return its current host port.
		if port := getContainerHostPort(mongoContainerName, "27017"); port != "" {
			return port, nil
		}
	}

	if status != "" {
		// Container exists but is stopped — restart it on its original port mapping.
		if err := exec.Command("docker", "start", mongoContainerName).Run(); err != nil {
			return "", fmt.Errorf("docker start: %w", err)
		}
		port := getContainerHostPort(mongoContainerName, "27017")
		if port == "" {
			return "", fmt.Errorf("could not determine MongoDB container port after restart")
		}
		if err := waitForPort("localhost", port, 20*time.Second); err != nil {
			return "", err
		}
		return port, nil
	}

	// Container does not exist — pick the port to bind.
	port := choosePort(preferredPort)
	fmt.Printf("   Starting MongoDB container on port %s...\n", port)
	args := []string{
		"run", "-d",
		"--name", mongoContainerName,
		"--restart", "unless-stopped",
		"-p", port + ":27017",
		"-e", "MONGO_INITDB_ROOT_USERNAME=" + user,
		"-e", "MONGO_INITDB_ROOT_PASSWORD=" + pass,
		mongoImage,
	}
	if out, err := exec.Command("docker", args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	if err := waitForPort("localhost", port, 20*time.Second); err != nil {
		return "", err
	}
	return port, nil
}

// ensureRedisRunning ensures the Redis container is running and returns the host port it listens on.
// preferredPort follows the same selection logic as ensureMongoRunning.
func ensureRedisRunning(pass, preferredPort string) (string, error) {
	status := containerStatus(redisContainerName)

	if status == "running" {
		if port := getContainerHostPort(redisContainerName, "6379"); port != "" {
			return port, nil
		}
	}

	if status != "" {
		if err := exec.Command("docker", "start", redisContainerName).Run(); err != nil {
			return "", fmt.Errorf("docker start: %w", err)
		}
		port := getContainerHostPort(redisContainerName, "6379")
		if port == "" {
			return "", fmt.Errorf("could not determine Redis container port after restart")
		}
		if err := waitForPort("localhost", port, 15*time.Second); err != nil {
			return "", err
		}
		return port, nil
	}

	port := choosePort(preferredPort)
	fmt.Printf("   Starting Redis container on port %s...\n", port)
	args := []string{
		"run", "-d",
		"--name", redisContainerName,
		"--restart", "unless-stopped",
		"-p", port + ":6379",
		redisImage,
		"redis-server", "--requirepass", pass,
	}
	if out, err := exec.Command("docker", args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	if err := waitForPort("localhost", port, 15*time.Second); err != nil {
		return "", err
	}
	return port, nil
}

// choosePort returns preferredPort if it is non-empty and not already bound by another process,
// otherwise falls back to a fresh random port.
func choosePort(preferredPort string) string {
	if preferredPort != "" && !isPortBound(preferredPort) {
		return preferredPort
	}
	port, err := freePort()
	if err != nil {
		// Should not happen; return a last-resort value.
		return "0"
	}
	return port
}

// isPortBound returns true if something is already listening on localhost:port.
func isPortBound(port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("localhost", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// infraSetupLoop is the interactive prompt shown when Docker is absent.
// It blocks until the user resolves the missing services or exits.
func infraSetupLoop(mongoOK, redisOK bool, mongoUser, mongoPass, mongoDB, redisPass string) error {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println()
		fmt.Println("──────────────────────────────────────────────────────────")
		fmt.Println("  DevTrack requires a database. Docker was not found.")
		fmt.Println("──────────────────────────────────────────────────────────")
		fmt.Println()
		fmt.Println("  [1] I have installed / started Docker — try again")
		fmt.Println("  [2] I have my own MongoDB / Redis — enter connection strings")
		fmt.Println("  [3] Exit (install Docker first, then run 'devtrack start')")
		fmt.Println()
		fmt.Println("  Docker Desktop: https://www.docker.com/products/docker-desktop")
		fmt.Println()
		fmt.Print("  Choice [1/2/3]: ")

		choice := readInfraLine(reader)

		switch choice {
		case "1", "":
			fmt.Println()
			fmt.Println("  Waiting for Docker... (press Ctrl-C to abort)")
			if err := waitForDocker(120 * time.Second); err != nil {
				fmt.Println("  ✗ Docker did not become available within 2 minutes.")
				fmt.Println("    Make sure Docker Desktop is running and try again.")
				continue
			}
			fmt.Println("  ✓ Docker is running.")
			return provisionContainers(mongoOK, redisOK, mongoUser, mongoPass, mongoDB, redisPass)

		case "2":
			if err := promptManualURIs(reader, &mongoOK, &redisOK); err != nil {
				fmt.Printf("  ✗ %v\n", err)
				continue
			}
			if mongoOK && redisOK {
				return nil
			}
			fmt.Println("  ✗ One or more services still not reachable. Try again.")

		case "3":
			fmt.Println()
			fmt.Println("  Exiting. Install Docker Desktop and run 'devtrack start' again.")
			fmt.Println("  https://www.docker.com/products/docker-desktop")
			return fmt.Errorf("database required — exiting setup")

		default:
			fmt.Println("  Invalid choice. Enter 1, 2, or 3.")
		}
	}
}

// waitForDocker polls dockerAvailable() until it returns true or the deadline passes.
func waitForDocker(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	dots := 0
	for time.Now().Before(deadline) {
		if dockerAvailable() {
			fmt.Println()
			return nil
		}
		<-ticker.C
		dots++
		fmt.Printf("\r  Waiting%s   ", strings.Repeat(".", dots%4))
	}
	fmt.Println()
	return fmt.Errorf("timeout")
}

// promptManualURIs reads MongoDB URI and Redis URL from the user and validates them.
func promptManualURIs(reader *bufio.Reader, mongoOK, redisOK *bool) error {
	if !*mongoOK {
		fmt.Println()
		fmt.Println("  MongoDB URI format: mongodb://user:pass@host:port/dbname")
		fmt.Print("  MONGODB_URI: ")
		uri := readInfraLine(reader)
		if uri == "" {
			return fmt.Errorf("MONGODB_URI cannot be empty")
		}
		host, port := parseMongoURI(uri)
		if !tcpPing(host, port) {
			return fmt.Errorf("cannot connect to MongoDB at %s:%s — check that the server is running", host, port)
		}
		os.Setenv("MONGODB_URI", uri)
		updateEnvFile("MONGODB_URI", uri)
		fmt.Println("  ✓ MongoDB connected")
		*mongoOK = true
	}

	if !*redisOK {
		fmt.Println()
		fmt.Println("  Redis URL format:   redis://:password@host:port/0")
		fmt.Print("  REDIS_URL: ")
		url := readInfraLine(reader)
		if url == "" {
			return fmt.Errorf("REDIS_URL cannot be empty")
		}
		host, port := parseRedisURL(url)
		if !tcpPing(host, port) {
			return fmt.Errorf("cannot connect to Redis at %s:%s — check that the server is running", host, port)
		}
		os.Setenv("REDIS_URL", url)
		updateEnvFile("REDIS_URL", url)
		fmt.Println("  ✓ Redis connected")
		*redisOK = true
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// freePort asks the OS for an available TCP port.
func freePort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	return port, err
}

// tcpPing returns true if host:port accepts a TCP connection within 2 s.
func tcpPing(host, port string) bool {
	if host == "" || port == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// dockerAvailable returns true if the Docker CLI is present and its daemon is reachable.
func dockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// getContainerHostPort returns the host port mapped to the given container internal port.
// e.g. getContainerHostPort("devtrack-mongo", "27017") → "58816"
func getContainerHostPort(containerName, internalPort string) string {
	out, err := exec.Command("docker", "port", containerName, internalPort).Output()
	if err != nil {
		return ""
	}
	// Output may contain multiple lines (IPv4 + IPv6): "0.0.0.0:58816\n:::58816"
	// Take the last non-empty line and split off the port.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		_, port, err := net.SplitHostPort(line)
		if err == nil && port != "" {
			return port
		}
	}
	return ""
}

// containerStatus returns the Docker container state string, or "" if it doesn't exist.
func containerStatus(name string) string {
	out, err := exec.Command("docker", "inspect", "--format={{.State.Status}}", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// waitForPort polls host:port until connectable or timeout.
func waitForPort(host, port string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if tcpPing(host, port) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("service at %s:%s not reachable after %s", host, port, timeout)
}

// printMissingServices prints which services are not reachable before prompting.
func printMissingServices(mongoOK, redisOK bool, mongoHost, mongoPort, redisHost, redisPort string) {
	fmt.Println()
	if !mongoOK {
		fmt.Printf("   ✗ MongoDB not reachable at %s:%s\n", mongoHost, mongoPort)
	}
	if !redisOK {
		fmt.Printf("   ✗ Redis not reachable at %s:%s\n", redisHost, redisPort)
	}
}

// resolveMongoConfig returns connection params for MongoDB from env vars.
func resolveMongoConfig() (host, port, user, pass, db string) {
	user = envOr("MONGO_USER", "devtrack")
	pass = envOr("MONGO_PASSWORD", "devtrack")
	port = envOr("MONGO_PORT", "27017")
	db = envOr("MONGODB_DB_NAME", "devtrack")

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		return "localhost", port, user, pass, db
	}
	h, p := parseMongoURI(uri)
	if h == "" {
		h = "localhost"
	}
	if p == "" {
		p = port
	}
	return h, p, user, pass, db
}

// resolveRedisConfig returns connection params for Redis from env vars.
func resolveRedisConfig() (host, port, pass string) {
	pass = envOr("REDIS_PASSWORD", "devtrack")
	port = envOr("REDIS_PORT", "6379")

	url := os.Getenv("REDIS_URL")
	if url == "" {
		return "localhost", port, pass
	}
	h, p := parseRedisURL(url)
	if h == "" {
		h = "localhost"
	}
	if p == "" {
		p = port
	}
	return h, p, pass
}

// parseMongoURI extracts host and port from mongodb://user:pass@host:port/db?...
func parseMongoURI(uri string) (host, port string) {
	s := strings.TrimPrefix(uri, "mongodb://")
	if i := strings.IndexByte(s, '?'); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndexByte(s, '@'); i >= 0 {
		s = s[i+1:]
	}
	return splitHostPort(s)
}

// parseRedisURL extracts host and port from redis://:pass@host:port/db
func parseRedisURL(url string) (host, port string) {
	s := strings.TrimPrefix(url, "redis://")
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndexByte(s, '@'); i >= 0 {
		s = s[i+1:]
	}
	return splitHostPort(s)
}

// splitHostPort splits "host:port" without stdlib restrictions on brackets.
func splitHostPort(s string) (host, port string) {
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// envOr returns the env var value, or fallback if empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// updateEnvFile replaces or appends key=value in the registered .env file.
func updateEnvFile(key, value string) {
	path := config.ResolveEnvFilePath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	prefix := key + "="
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = key + "=" + value
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, key+"="+value)
	}
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
}

// readInfraLine reads a trimmed line from stdin.
func readInfraLine(r *bufio.Reader) string {
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}

// CheckInfraStatus prints MongoDB/Redis reachability — used by `devtrack status`.
func CheckInfraStatus() {
	mongoHost, mongoPort, _, _, _ := resolveMongoConfig()
	redisHost, redisPort, _ := resolveRedisConfig()

	mongoIcon := "✓"
	if !tcpPing(mongoHost, mongoPort) {
		mongoIcon = "✗"
	}
	redisIcon := "✓"
	if !tcpPing(redisHost, redisPort) {
		redisIcon = "✗"
	}
	fmt.Printf("   MongoDB  %s  %s:%s\n", mongoIcon, mongoHost, mongoPort)
	fmt.Printf("   Redis    %s  %s:%s\n", redisIcon, redisHost, redisPort)
}
