package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	pb "github.com/kytheron-org/kytheron-plugin-go/plugin"
	"google.golang.org/grpc"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

type PluginType string

const (
	PluginTypeSource PluginType = "source"
	PluginTypeParser PluginType = "parser"
	PluginTypeOutput PluginType = "output"
)

// Plugin Registry Configuration
type PluginRegistry struct {
	BaseURL   string // e.g., "https://plugins.example.com"
	CacheDir  string // Local directory to cache plugins
	IndexURL  string // URL to plugin index file
	mu        sync.RWMutex
	plugins   map[string]pb.PluginClient
	parsers   map[string]pb.ParserPluginClient
	outputs   map[string]pb.OutputPluginClient
	processes map[string]*exec.Cmd
}

// Plugin Manifest (similar to Terraform's provider manifest)
type PluginManifest struct {
	Name     string            `json:"name"`
	Version  string            `json:"version"`
	Binaries map[string]Binary `json:"binaries"` // OS-arch mapping
}

type Binary struct {
	URL      string `json:"url"`
	Checksum string `json:"checksum"` // SHA256
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry(cacheDir string) *PluginRegistry {
	if cacheDir == "" {
		homeDir, _ := os.UserHomeDir()
		cacheDir = filepath.Join(homeDir, ".logprocessor", "plugins")
	}

	return &PluginRegistry{
		BaseURL:   "https://github.com/kytheron-org",
		CacheDir:  cacheDir,
		IndexURL:  "https://plugins.example.com/index.json",
		parsers:   make(map[string]pb.ParserPluginClient),
		outputs:   make(map[string]pb.OutputPluginClient),
		plugins:   make(map[string]pb.PluginClient),
		processes: make(map[string]*exec.Cmd),
	}
}

func (r *PluginRegistry) Parser(name string) (pb.ParserPluginClient, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	val, ok := r.parsers[name]
	if !ok {
		return nil, fmt.Errorf("no parser plugin client for %s", name)
	}
	return val, nil
}

func (r *PluginRegistry) DownloadPlugin(ctx context.Context, manifest PluginManifest) (string, error) {
	// Determine OS and architecture
	osArch := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
	binary, ok := manifest.Binaries[osArch]
	if !ok {
		return "", fmt.Errorf("no binary available for %s", osArch)
	}

	// Check if already cached and valid
	pluginDir := filepath.Join(r.CacheDir, manifest.Name, manifest.Version)
	pluginPath := filepath.Join(pluginDir, manifest.Name)
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}

	if r.isPluginCached(pluginPath, binary.Checksum) {
		log.Printf("Plugin %s@%s already cached\n", manifest.Name, manifest.Version)
		return pluginPath, nil
	}

	// Create plugin directory
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Download the plugin
	log.Printf("Downloading plugin %s@%s from %s\n", manifest.Name, manifest.Version, binary.URL)

	req, err := http.NewRequestWithContext(ctx, "GET", binary.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Write to temp file and verify checksum
	tempPath := pluginPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(tempFile, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to write plugin: %w", err)
	}

	// TODO: Verify checksum
	//actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	//if actualChecksum != binary.Checksum {
	//	os.Remove(tempPath)
	//	return "", fmt.Errorf("checksum mismatch: expected %s, got %s", binary.Checksum, actualChecksum)
	//}

	// Make executable and move to final location
	if err := os.Chmod(tempPath, 0755); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to make plugin executable: %w", err)
	}

	if err := os.Rename(tempPath, pluginPath); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to move plugin: %w", err)
	}

	log.Printf("Successfully downloaded plugin %s@%s\n", manifest.Name, manifest.Version)
	return pluginPath, nil
}

func (r *PluginRegistry) isPluginCached(path, expectedChecksum string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	return actualChecksum == expectedChecksum
}

// StartPlugin launches a plugin process and establishes gRPC connection
func (r *PluginRegistry) StartPlugin(ctx context.Context, pluginPath string, pluginType PluginType) (string, error) {
	// Start the plugin process
	// The plugin will print its gRPC address to stdout in the format: "1|1|tcp|127.0.0.1:12345|grpc"
	cmd := exec.CommandContext(ctx, pluginPath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start logging stderr
	go func() {
		scanner := io.Reader(stderr)
		buf := make([]byte, 1024)
		for {
			n, err := scanner.Read(buf)
			if n > 0 {
				log.Printf("[plugin stderr] %s", string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start plugin: %w", err)
	}

	// Store process for cleanup
	r.mu.Lock()
	r.processes[pluginPath] = cmd
	r.mu.Unlock()

	// Read the handshake from stdout
	handshake := make([]byte, 1024)
	n, err := stdout.Read(handshake)
	if err != nil {
		cmd.Process.Kill()
		return "", fmt.Errorf("failed to read handshake: %w", err)
	}

	// Parse handshake (simplified - real implementation would parse the protocol properly)
	// Format: "1|1|tcp|127.0.0.1:12345|grpc"
	address, err := parseHandshake(string(handshake[:n]))
	if err != nil || address == "" {
		cmd.Process.Kill()
		return "", fmt.Errorf("invalid handshake format")
	}

	log.Printf("Plugin started on %s\n", address)
	return address, nil
}

func parseHandshake(handshake string) (string, error) {
	var hs map[string]interface{}
	err := json.Unmarshal([]byte(handshake), &hs)
	if err != nil {
		return "", err
	}

	msgType, ok := hs["type"]
	if !ok || msgType != "handshake" {
		return "", fmt.Errorf("handshake message type not handshake")
	}

	addr, ok := hs["addr"]
	if !ok {
		return "", fmt.Errorf("handshake message address not provided")
	}

	return addr.(string), nil
}

func (r *PluginRegistry) LoadPlugin(ctx context.Context, name, version string) error {
	r.mu.RLock()
	if _, ok := r.plugins[name]; ok {
		r.mu.RUnlock()
		return nil
	}

	r.mu.RUnlock()
	// TODO: Download checksums
	manifest := PluginManifest{
		Name:    name,
		Version: version,
		Binaries: map[string]Binary{
			fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH): {
				URL:      fmt.Sprintf("%s/kytheron-plugin-%s/releases/download/%s/kytheron-plugin-%s_%s_%s", r.BaseURL, name, version, name, runtime.GOOS, runtime.GOARCH),
				Checksum: "d52eab3db33b5b19a41b42f5b776f2ceebf74981fba8ded9bef2743d75f50471",
			},
		},
	}

	pluginPath, err := r.DownloadPlugin(ctx, manifest)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}

	address, err := r.StartPlugin(ctx, pluginPath, PluginTypeParser)
	if err != nil {
		return fmt.Errorf("failed to start plugin: %w", err)
	}

	conn, err := grpc.NewClient(fmt.Sprintf("unix://%s", address), grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to plugin: %w", err)
	}
	client := pb.NewPluginClient(conn)

	r.mu.Lock()
	r.plugins[name] = client
	r.mu.Unlock()

	// We also need to get the supported interfaces
	//meta, err := client.GetMetadata(context.TODO(), &pb.Empty{})
	//if err != nil {
	//	r.mu.Lock()
	//delete(r.plugins, name)
	//r.mu.Unlock()
	//return err
	//}

	// TODO: Plugins need to implement the metadata endpoints
	//for _, metaType := range meta.GetTypes() {
	//	switch metaType {
	//	case "parser":
	r.mu.Lock()
	r.parsers[name] = pb.NewParserPluginClient(conn)
	r.mu.Unlock()
	//case "output":
	r.mu.Lock()
	r.outputs[name] = pb.NewOutputPluginClient(conn)
	r.mu.Unlock()
	//}
	//}

	return nil
}

func (r *PluginRegistry) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()

	//Close all source clients
	//for name, client := range r.parsers {
	//	if err := client.Close(); err != nil {
	//		log.Printf("Error closing source plugin %s: %v\n", name, err)
	//	}
	//}
	//
	//// Close all parser clients
	//for name, client := range r.parsers {
	//	if err := client.Close(); err != nil {
	//		log.Printf("Error closing parser plugin %s: %v\n", name, err)
	//	}
	//}

	// Kill all processes
	for path, cmd := range r.processes {
		log.Printf("Stopping plugin process: %s\n", path)
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}

	r.parsers = make(map[string]pb.ParserPluginClient)
	r.outputs = make(map[string]pb.OutputPluginClient)
	r.plugins = make(map[string]pb.PluginClient)
	r.processes = make(map[string]*exec.Cmd)
}
