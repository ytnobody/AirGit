package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

const version = "1.0.0"

//go:embed static/*
var staticFiles embed.FS

type Config struct {
	RepoPath   string
	ListenAddr string
	ListenPort string
	TLSCert    string
	TLSKey     string
}

type Response struct {
	Branch   string      `json:"branch,omitempty"`
	RepoName string      `json:"repoName,omitempty"`
	Error    string      `json:"error,omitempty"`
	Log      []string    `json:"log,omitempty"`
	Commit   string      `json:"commit,omitempty"`
	Branches []string    `json:"branches,omitempty"`
	Tags     []string    `json:"tags,omitempty"`
	Remotes  []string    `json:"remotes,omitempty"`
	Ahead    int         `json:"ahead,omitempty"`
	Behind   int         `json:"behind,omitempty"`
	Commits  []CommitInfo `json:"commits,omitempty"`
}

type CommitInfo struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

type RemoteInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Repository struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type AgentStatus struct {
	IssueNumber int       `json:"issueNumber"`
	Status      string    `json:"status"` // "pending", "running", "completed", "failed"
	Message     string    `json:"message"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime,omitempty"`
	PRNumber    int       `json:"prNumber,omitempty"`
}

var config Config
var baseRepoPath string
var agentStatus map[int]AgentStatus // issueNumber -> status
var agentStatusMutex sync.Mutex

func init() {
	config = Config{
		RepoPath:   getEnv("AIRGIT_REPO_PATH", os.Getenv("HOME")),
		ListenAddr: getEnv("AIRGIT_LISTEN_ADDR", "0.0.0.0"),
		ListenPort: getEnv("AIRGIT_LISTEN_PORT", "8080"),
		TLSCert:    getEnv("AIRGIT_TLS_CERT", ""),
		TLSKey:     getEnv("AIRGIT_TLS_KEY", ""),
	}
	baseRepoPath = config.RepoPath
	agentStatus = make(map[int]AgentStatus)

	log.Printf("Config: RepoPath=%s", config.RepoPath)
}

func getEnv(key, defaultVal string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultVal
}

func main() {
	var showHelp bool
	var showVersion bool
	var repoPath string
	var listenAddr string
	var listenPort string
	var tlsCert string
	var tlsKey string

	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.BoolVar(&showHelp, "h", false, "Show help message (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.BoolVar(&showVersion, "v", false, "Show version (shorthand)")
	flag.StringVar(&repoPath, "repo-path", "", "Absolute path to Git repository (default: $HOME)")
	flag.StringVar(&listenAddr, "listen-addr", "", "Server listen address (default: 0.0.0.0)")
	flag.StringVar(&listenPort, "listen-port", "", "Server listen port (default: 8080)")
	flag.StringVar(&listenPort, "port", "", "Server listen port (alias for --listen-port, default: 8080)")
	flag.StringVar(&listenPort, "p", "", "Server listen port (shorthand, default: 8080)")
	flag.StringVar(&tlsCert, "tls-cert", "", "Path to TLS certificate file (for HTTPS)")
	flag.StringVar(&tlsKey, "tls-key", "", "Path to TLS key file (for HTTPS)")

	flag.Parse()

	if showHelp {
		printHelp()
		os.Exit(0)
	}

	if showVersion {
		fmt.Printf("AirGit version %s\n", version)
		os.Exit(0)
	}

	// Override config with command-line flags if provided
	if repoPath != "" {
		config.RepoPath = repoPath
	}
	if listenAddr != "" {
		config.ListenAddr = listenAddr
	}
	if listenPort != "" {
		config.ListenPort = listenPort
	}
	if tlsCert != "" {
		config.TLSCert = tlsCert
	}
	if tlsKey != "" {
		config.TLSKey = tlsKey
	}

	http.HandleFunc("/manifest.json", serveManifest)
	http.HandleFunc("/service-worker.js", serveServiceWorker)
	http.HandleFunc("/icon.png", serveIcon)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/push", handlePush)
	http.HandleFunc("/api/pull", handlePull)
	http.HandleFunc("/api/commits", handleListCommits)
	http.HandleFunc("/api/repos", handleListRepos)
	http.HandleFunc("/api/load-repo", handleLoadRepo)
	http.HandleFunc("/api/branch/create", handleCreateBranch)
	http.HandleFunc("/api/branches", handleListBranches)
	http.HandleFunc("/api/checkout", handleCheckoutBranch)
	http.HandleFunc("/api/repo/create", handleCreateRepo)
	http.HandleFunc("/api/repo/init", handleInitRepo)
	http.HandleFunc("/api/remotes", handleListRemotes)
	http.HandleFunc("/api/remote/add", handleAddRemote)
	http.HandleFunc("/api/remote/update", handleUpdateRemote)
	http.HandleFunc("/api/remote/remove", handleRemoveRemote)
	http.HandleFunc("/api/tags", handleListTags)
	http.HandleFunc("/api/tag/create", handleCreateTag)
	http.HandleFunc("/api/tag/push", handlePushTag)
	http.HandleFunc("/api/systemd/register", handleSystemdRegister)
	http.HandleFunc("/api/systemd/status", handleSystemdStatus)
	http.HandleFunc("/api/systemd/service-status", handleSystemdServiceStatus)
	http.HandleFunc("/api/systemd/service-start", handleSystemdServiceStart)
	http.HandleFunc("/api/github/issues", handleListGitHubIssues)
	http.HandleFunc("/api/agent/trigger", handleAgentTrigger)
	http.HandleFunc("/api/agent/process", handleAgentProcess)
	http.HandleFunc("/api/agent/status", handleAgentStatus)
	http.HandleFunc("/", serveRoot)

	addr := net.JoinHostPort(config.ListenAddr, config.ListenPort)
	
	// Determine if using TLS
	if config.TLSCert != "" && config.TLSKey != "" {
		log.Printf("Starting AirGit on https://%s (with TLS)", addr)
		if err := http.ListenAndServeTLS(addr, config.TLSCert, config.TLSKey, nil); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("Starting AirGit on http://%s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatal(err)
		}
	}
}

func printHelp() {
	fmt.Printf(`AirGit - Lightweight web-based Git GUI for mobile devices

Usage: airgit [options]

Options:
  -h, --help                Show this help message
  -v, --version             Show version information
  --repo-path <path>        Absolute path to Git repository (env: AIRGIT_REPO_PATH, default: $HOME)
  --listen-addr <addr>      Server listen address (env: AIRGIT_LISTEN_ADDR, default: 0.0.0.0)
  -p, --port, --listen-port <port>
                            Server listen port (env: AIRGIT_LISTEN_PORT, default: 8080)
  --tls-cert <path>         Path to TLS certificate file (env: AIRGIT_TLS_CERT, for HTTPS)
  --tls-key <path>          Path to TLS key file (env: AIRGIT_TLS_KEY, for HTTPS)

Examples:
  # Using environment variables
  export AIRGIT_REPO_PATH=/path/to/repo
  airgit

  # Using command-line flags
  airgit --repo-path /path/to/repo

  # With HTTPS (requires certificate and key files)
  airgit --tls-cert /path/to/cert.pem --tls-key /path/to/key.pem

  # Using port option
  airgit -p 3000
  airgit --port 3000

  # Show help and version
  airgit --help
  airgit --version

For more information, visit: https://github.com/your-repo/AirGit
`)
}

func serveRoot(w http.ResponseWriter, r *http.Request) {
	// Always serve index.html - the frontend will handle routing
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
			return
		}
		w.Write(data)
		return
	}
	http.NotFound(w, r)
}

func serveManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	data, err := staticFiles.ReadFile("static/manifest.json")
	if err != nil {
		http.Error(w, "Failed to read manifest.json", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

func serveServiceWorker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	data, err := staticFiles.ReadFile("static/service-worker.js")
	if err != nil {
		http.Error(w, "Failed to read service-worker.js", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

func serveIcon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	data, err := staticFiles.ReadFile("static/icon.png")
	if err != nil {
		http.Error(w, "Failed to read icon.png", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	repoPath := r.URL.Query().Get("repoPath")
	
	// Use provided repoPath or fall back to config.RepoPath
	originalRepoPath := config.RepoPath
	if repoPath != "" {
		// Resolve and validate the path
		var resolvedPath string
		var err error
		if filepath.IsAbs(repoPath) {
			resolvedPath = repoPath
		} else {
			resolvedPath = filepath.Join(originalRepoPath, repoPath)
		}
		resolvedPath, err = filepath.Abs(resolvedPath)
		if err == nil {
			basePath, _ := filepath.Abs(originalRepoPath)
			if strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) || resolvedPath == basePath {
				config.RepoPath = resolvedPath
			}
		}
	}

	defer func() {
		config.RepoPath = originalRepoPath
	}()

	branch, err := executeGitCommand("branch", "--show-current")
	if err != nil || strings.TrimSpace(branch) == "" {
		// Fallback: try alternative method to get current branch
		branch, err = executeGitCommand("rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			json.NewEncoder(w).Encode(Response{
				Error: fmt.Sprintf("Failed to get branch: %v", err),
			})
			return
		}
	}

	branch = strings.TrimSpace(branch)

	// Get repository name from the directory name
	repoName := filepath.Base(config.RepoPath)

	// Get ahead/behind count
	ahead, behind := getAheadBehind(branch)

	json.NewEncoder(w).Encode(Response{
		Branch:   branch,
		RepoName: repoName,
		Ahead:    ahead,
		Behind:   behind,
	})
}

func handlePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	repoPath := r.URL.Query().Get("repoPath")
	remote := r.URL.Query().Get("remote")
	if remote == "" {
		remote = "origin"
	}
	
	// Use provided repoPath or fall back to config.RepoPath
	originalRepoPath := config.RepoPath
	if repoPath != "" {
		// Resolve and validate the path
		var resolvedPath string
		var err error
		if filepath.IsAbs(repoPath) {
			resolvedPath = repoPath
		} else {
			resolvedPath = filepath.Join(originalRepoPath, repoPath)
		}
		resolvedPath, err = filepath.Abs(resolvedPath)
		if err == nil {
			basePath, _ := filepath.Abs(originalRepoPath)
			if strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) || resolvedPath == basePath {
				config.RepoPath = resolvedPath
			}
		}
	}

	defer func() {
		config.RepoPath = originalRepoPath
	}()

	var logs []string

	// Get current branch
	branch, err := executeGitCommand("branch", "--show-current")
	if err != nil || strings.TrimSpace(branch) == "" {
		// Fallback: try alternative method
		branch, err = executeGitCommand("rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			resp := Response{
				Error: fmt.Sprintf("Failed to get branch: %v", err),
				Log:   logs,
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}

	branch = strings.TrimSpace(branch)

	// git push [remote] [branch]
	output, err := executeGitCommand("push", remote, branch)
	logs = append(logs, fmt.Sprintf("$ git push %s %s", remote, branch))
	if output != "" {
		logs = append(logs, output)
	}
	if err != nil {
		// Check if it's "everything up-to-date" which is not really an error
		if strings.Contains(output, "up to date") || strings.Contains(output, "up-to-date") {
			logs = append(logs, "✓ Everything is already up to date!")
		} else {
			resp := Response{
				Error: fmt.Sprintf("git push failed: %v", err),
				Log:   logs,
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(resp)
			return
		}
	} else {
		logs = append(logs, "✓ Push successful!")
	}

	json.NewEncoder(w).Encode(Response{
		Branch: branch,
		Log:    logs,
	})
}

func handlePull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	repoPath := r.URL.Query().Get("repoPath")
	remote := r.URL.Query().Get("remote")
	if remote == "" {
		remote = "origin"
	}
	
	// Use provided repoPath or fall back to config.RepoPath
	originalRepoPath := config.RepoPath
	if repoPath != "" {
		// Resolve and validate the path
		var resolvedPath string
		var err error
		if filepath.IsAbs(repoPath) {
			resolvedPath = repoPath
		} else {
			resolvedPath = filepath.Join(originalRepoPath, repoPath)
		}
		resolvedPath, err = filepath.Abs(resolvedPath)
		if err == nil {
			basePath, _ := filepath.Abs(originalRepoPath)
			if strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) || resolvedPath == basePath {
				config.RepoPath = resolvedPath
			}
		}
	}

	defer func() {
		config.RepoPath = originalRepoPath
	}()

	var logs []string

	// Get current branch
	branch, err := executeGitCommand("branch", "--show-current")
	if err != nil || strings.TrimSpace(branch) == "" {
		// Fallback: try alternative method
		branch, err = executeGitCommand("rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			resp := Response{
				Error: fmt.Sprintf("Failed to get branch: %v", err),
				Log:   logs,
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}

	branch = strings.TrimSpace(branch)

	// git pull [remote] [branch]
	output, err := executeGitCommand("pull", remote, branch)
	logs = append(logs, fmt.Sprintf("$ git pull %s %s", remote, branch))
	if output != "" {
		logs = append(logs, output)
	}
	if err != nil {
		resp := Response{
			Error: fmt.Sprintf("git pull failed: %v", err),
			Log:   logs,
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	logs = append(logs, "✓ Pull successful!")

	json.NewEncoder(w).Encode(Response{
		Branch: branch,
		Log:    logs,
	})
}


func handleListRepos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	log.Printf("ListRepos: scanning path=%s", baseRepoPath)
	repos, err := listRepositories(baseRepoPath)
	if err != nil {
		log.Printf("ListRepos error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("Failed to list repos: %v", err),
		})
		return
	}

	log.Printf("ListRepos: found %d repositories", len(repos))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"repositories": repos,
	})
}

func handleSelectRepo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		RelativePath string `json:"relativePath"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	if req.RelativePath == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Relative path is required",
		})
		return
	}

	// Resolve the relative path safely to prevent directory traversal
	resolvedPath := filepath.Join(baseRepoPath, req.RelativePath)
	resolvedPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid path",
		})
		return
	}

	// Check that resolved path is within the base repo path
	basePath, _ := filepath.Abs(baseRepoPath)
	if !strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) && resolvedPath != basePath {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Path traversal not allowed",
		})
		return
	}

	config.RepoPath = resolvedPath

	// Load status after changing repo
	branch, err := executeGitCommand("branch", "--show-current")
	if err != nil || strings.TrimSpace(branch) == "" {
		// Fallback: try alternative method
		branch, err = executeGitCommand("rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": fmt.Sprintf("Failed to get branch: %v", err),
			})
			return
		}
	}

	branch = strings.TrimSpace(branch)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"branch": branch,
	})
}

func handleCreateBranch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		BranchName string `json:"branchName"`
		Checkout   bool   `json:"checkout"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Invalid request body",
		})
		return
	}

	if req.BranchName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Branch name is required",
		})
		return
	}

	var logs []string
	var args []string

	if req.Checkout {
		args = []string{"checkout", "-b", req.BranchName}
		logs = append(logs, fmt.Sprintf("$ git checkout -b %s", req.BranchName))
	} else {
		args = []string{"branch", req.BranchName}
		logs = append(logs, fmt.Sprintf("$ git branch %s", req.BranchName))
	}

	output, err := executeGitCommand(args...)
	if output != "" {
		logs = append(logs, output)
	}
	if err != nil {
		resp := Response{
			Error: fmt.Sprintf("Failed to create branch: %v", err),
			Log:   logs,
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	if req.Checkout {
		logs = append(logs, fmt.Sprintf("✓ Branch '%s' created and checked out!", req.BranchName))
	} else {
		logs = append(logs, fmt.Sprintf("✓ Branch '%s' created!", req.BranchName))
	}

	json.NewEncoder(w).Encode(Response{
		Branch: req.BranchName,
		Log:    logs,
	})
}

func executeGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = config.RepoPath

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()

	result := strings.TrimSpace(output.String())

	// Some git commands exit with non-zero but still succeed
	if err != nil {
		// Check if output contains error keywords
		if result != "" {
			return result, err
		}
		return "", err
	}

	return result, nil
}

func getAheadBehind(branch string) (int, int) {
	// Try to get the tracking branch
	trackingBranch, err := executeGitCommand("rev-parse", "--abbrev-ref", branch+"@{u}")
	if err != nil || strings.TrimSpace(trackingBranch) == "" {
		// No tracking branch configured
		return 0, 0
	}

	trackingBranch = strings.TrimSpace(trackingBranch)

	// Get ahead/behind count
	output, err := executeGitCommand("rev-list", "--left-right", "--count", branch+"..."+trackingBranch)
	if err != nil {
		return 0, 0
	}

	parts := strings.Fields(strings.TrimSpace(output))
	if len(parts) >= 2 {
		var ahead, behind int
		fmt.Sscanf(parts[0], "%d", &ahead)
		fmt.Sscanf(parts[1], "%d", &behind)
		return ahead, behind
	}

	return 0, 0
}


func listRepositories(basePath string) ([]Repository, error) {
	var repos []Repository

	// Check if basePath exists and is a directory
	info, err := os.Stat(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to access repository path: %v", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("repository path is not a directory: %s", basePath)
	}

	absBasePath, _ := filepath.Abs(basePath)

	err = filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip directories we can't access instead of failing completely
			return nil
		}

		if info == nil {
			return nil
		}

		// Check for .git directory first
		if info.Name() == ".git" && info.IsDir() {
			repoPath := filepath.Dir(path)
			name := filepath.Base(repoPath)

			// Calculate relative path from basePath
			relPath, err := filepath.Rel(absBasePath, repoPath)
			if err != nil {
				relPath = repoPath
			}

			repos = append(repos, Repository{
				Name: name,
				Path: relPath,
			})

			return filepath.SkipDir
		}

		// Skip hidden directories (except we already handled .git)
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %v", err)
	}

	// Return empty slice instead of nil
	if repos == nil {
		repos = []Repository{}
	}

	return repos, nil
}

func handleListBranches(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	log.Printf("handleListBranches: RepoPath=%s", config.RepoPath)

	output, err := executeGitCommand("branch", "-a")
	if err != nil {
		log.Printf("handleListBranches error: %v, output=%s", err, output)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Failed to list branches: %v", err),
		})
		return
	}

	var branches []string
	branchMap := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "remotes/")
		if branchMap[line] {
			continue
		}
		branchMap[line] = true
		branches = append(branches, line)
	}

	json.NewEncoder(w).Encode(Response{
		Branches: branches,
	})
}

func handleCheckoutBranch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Branch string `json:"branch"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Invalid request body",
		})
		return
	}

	if req.Branch == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Branch name is required",
		})
		return
	}

	output, err := executeGitCommand("checkout", req.Branch)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Failed to checkout branch: %v", err),
			Log:   []string{output},
		})
		return
	}

	branch, _ := executeGitCommand("branch", "--show-current")
	branch = strings.TrimSpace(branch)

	ahead, behind := getAheadBehind(branch)

	json.NewEncoder(w).Encode(Response{
		Branch: branch,
		Ahead:  ahead,
		Behind: behind,
		Log:    []string{fmt.Sprintf("Switched to branch: %s", branch)},
	})
}

func handleLoadRepo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodPost {
		// POST: select a repository from the request body
		var req struct {
			RelativePath string `json:"relativePath"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		if req.RelativePath == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Relative path is required",
			})
			return
		}

		// Resolve the relative path safely to prevent directory traversal
		resolvedPath := filepath.Join(baseRepoPath, req.RelativePath)
		resolvedPath, err := filepath.Abs(resolvedPath)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid path",
			})
			return
		}

		// Check that resolved path is within the base repo path
		basePath, _ := filepath.Abs(baseRepoPath)
		if !strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) && resolvedPath != basePath {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Path traversal not allowed",
			})
			return
		}

		config.RepoPath = resolvedPath
	} else {
		// GET: load repository and branch from query parameters
		relativePath := r.URL.Query().Get("p")
		branch := r.URL.Query().Get("b")

		// If relativePath is provided, update repo path safely
		if relativePath != "" {
			// Resolve the relative path safely to prevent directory traversal
			resolvedPath := filepath.Join(baseRepoPath, relativePath)
			resolvedPath, err := filepath.Abs(resolvedPath)
			if err == nil {
				// Check that resolved path is within the base repo path
				basePath, _ := filepath.Abs(baseRepoPath)
				if strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) || resolvedPath == basePath {
					config.RepoPath = resolvedPath
				}
			}
		}

		// If branch is provided, checkout that branch
		if branch != "" {
			_, _ = executeGitCommand("checkout", branch)
		}
	}

	// Get current branch
	currentBranch, err := executeGitCommand("branch", "--show-current")
	if err != nil || strings.TrimSpace(currentBranch) == "" {
		currentBranch, err = executeGitCommand("rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": fmt.Sprintf("Failed to get branch: %v", err),
			})
			return
		}
	}

	currentBranch = strings.TrimSpace(currentBranch)

	// Get repository name from the directory name
	repoName := filepath.Base(config.RepoPath)

	// Get ahead/behind count
	ahead, behind := getAheadBehind(currentBranch)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"branch":   currentBranch,
		"repoName": repoName,
		"ahead":    ahead,
		"behind":   behind,
	})
}

func handleInit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	relativePath := r.URL.Query().Get("relativePath")
	branch := r.URL.Query().Get("branch")

	// If relativePath is provided, update repo path safely
	if relativePath != "" {
		// Resolve the relative path safely to prevent directory traversal
		resolvedPath := filepath.Join(baseRepoPath, relativePath)
		resolvedPath, err := filepath.Abs(resolvedPath)
		if err == nil {
			// Check that resolved path is within the base repo path
			basePath, _ := filepath.Abs(baseRepoPath)
			if strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) || resolvedPath == basePath {
				config.RepoPath = resolvedPath
			}
		}
	}

	// Get current branch
	currentBranch, err := executeGitCommand("branch", "--show-current")
	if err != nil || strings.TrimSpace(currentBranch) == "" {
		currentBranch, err = executeGitCommand("rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": fmt.Sprintf("Failed to get branch: %v", err),
			})
			return
		}
	}

	currentBranch = strings.TrimSpace(currentBranch)

	// If branch is provided and different from current, checkout that branch
	if branch != "" && branch != currentBranch {
		output, err := executeGitCommand("checkout", branch)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": fmt.Sprintf("Failed to checkout branch '%s': %v. Output: %s", branch, err, output),
				"branch": currentBranch,
			})
			return
		}
		currentBranch = branch
	}

	// Get ahead/behind count
	ahead, behind := getAheadBehind(currentBranch)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"branch": currentBranch,
		"ahead":  ahead,
		"behind": behind,
	})
}

func handleCreateRepo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		RepoName  string `json:"repoName"`
		Subdirs   string `json:"subdirs"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Invalid request body",
		})
		return
	}

	if req.RepoName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Repository name is required",
		})
		return
	}

	// Resolve the repository path safely
	resolvedPath := filepath.Join(baseRepoPath, req.RepoName)
	resolvedPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Invalid path",
		})
		return
	}

	// Check that resolved path is within the base repo path
	basePath, _ := filepath.Abs(baseRepoPath)
	if !strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) && resolvedPath != basePath {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Path traversal not allowed",
		})
		return
	}

	// Check if directory already exists
	if _, err := os.Stat(resolvedPath); err == nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Repository directory already exists: %s", req.RepoName),
		})
		return
	}

	// Create the repository directory
	if err := os.MkdirAll(resolvedPath, 0755); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Failed to create directory: %v", err),
		})
		return
	}

	// Create subdirectories if specified
	if req.Subdirs != "" {
		subdirs := strings.Split(req.Subdirs, ",")
		for _, subdir := range subdirs {
			subdir = strings.TrimSpace(subdir)
			if subdir != "" {
				subdirPath := filepath.Join(resolvedPath, subdir)
				if err := os.MkdirAll(subdirPath, 0755); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(Response{
						Error: fmt.Sprintf("Failed to create subdirectory '%s': %v", subdir, err),
					})
					return
				}
			}
		}
	}

	// Change to the new directory and initialize git repository
	originalRepoPath := config.RepoPath
	config.RepoPath = resolvedPath
	defer func() {
		config.RepoPath = originalRepoPath
	}()

	var logs []string

	// Initialize git repository
	output, err := executeGitCommand("init")
	logs = append(logs, "$ git init")
	if output != "" {
		logs = append(logs, output)
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Failed to initialize git repository: %v", err),
			Log:   logs,
		})
		return
	}

	logs = append(logs, fmt.Sprintf("✓ Repository '%s' created and initialized!", req.RepoName))

	json.NewEncoder(w).Encode(Response{
		RepoName: req.RepoName,
		Log:      logs,
	})
}

func handleInitRepo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		RepoPath string `json:"repoPath"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Invalid request body",
		})
		return
	}

	if req.RepoPath == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Repository path is required",
		})
		return
	}

	// Resolve the repository path safely
	resolvedPath := filepath.Join(baseRepoPath, req.RepoPath)
	resolvedPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Invalid path",
		})
		return
	}

	// Check that resolved path is within the base repo path
	basePath, _ := filepath.Abs(baseRepoPath)
	if !strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) && resolvedPath != basePath {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Path traversal not allowed",
		})
		return
	}

	// Check if directory exists
	info, err := os.Stat(resolvedPath)
	if err != nil || !info.IsDir() {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Directory does not exist: %s", req.RepoPath),
		})
		return
	}

	// Check if already a git repository
	gitDir := filepath.Join(resolvedPath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Directory is already a git repository: %s", req.RepoPath),
		})
		return
	}

	// Change to the directory and initialize git repository
	originalRepoPath := config.RepoPath
	config.RepoPath = resolvedPath
	defer func() {
		config.RepoPath = originalRepoPath
	}()

	var logs []string

	// Initialize git repository
	output, err := executeGitCommand("init")
	logs = append(logs, "$ git init")
	if output != "" {
		logs = append(logs, output)
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Failed to initialize git repository: %v", err),
			Log:   logs,
		})
		return
	}

	repoName := filepath.Base(resolvedPath)
	logs = append(logs, fmt.Sprintf("✓ Repository '%s' initialized!", repoName))

	json.NewEncoder(w).Encode(Response{
		RepoName: repoName,
		Log:      logs,
	})
}

func handleListRemotes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	repoPath := r.URL.Query().Get("repoPath")
	
	// Use provided repoPath or fall back to config.RepoPath
	originalRepoPath := config.RepoPath
	if repoPath != "" {
		// Resolve and validate the path
		var resolvedPath string
		var err error
		if filepath.IsAbs(repoPath) {
			resolvedPath = repoPath
		} else {
			resolvedPath = filepath.Join(originalRepoPath, repoPath)
		}
		resolvedPath, err = filepath.Abs(resolvedPath)
		if err == nil {
			basePath, _ := filepath.Abs(originalRepoPath)
			if strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) || resolvedPath == basePath {
				config.RepoPath = resolvedPath
			}
		}
	}

	defer func() {
		config.RepoPath = originalRepoPath
	}()

	output, err := executeGitCommand("remote", "-v")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Failed to list remotes: %v", err),
		})
		return
	}

	var remotes []RemoteInfo
	remoteMap := make(map[string]string)
	
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[0]
			url := parts[1]
			if remoteMap[name] == "" {
				remoteMap[name] = url
			}
		}
	}
	
	for name, url := range remoteMap {
		remotes = append(remotes, RemoteInfo{
			Name: name,
			URL:  url,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"remotes": remotes,
	})
}

func handleAddRemote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	repoPath := r.URL.Query().Get("repoPath")

	// Use provided repoPath or fall back to config.RepoPath
	originalRepoPath := config.RepoPath
	if repoPath != "" {
		// Resolve and validate the path
		var resolvedPath string
		var err error
		if filepath.IsAbs(repoPath) {
			resolvedPath = repoPath
		} else {
			resolvedPath = filepath.Join(originalRepoPath, repoPath)
		}
		resolvedPath, err = filepath.Abs(resolvedPath)
		if err == nil {
			basePath, _ := filepath.Abs(originalRepoPath)
			if strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) || resolvedPath == basePath {
				config.RepoPath = resolvedPath
			}
		}
	}

	defer func() {
		config.RepoPath = originalRepoPath
	}()

	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Invalid request body",
		})
		return
	}

	if req.Name == "" || req.URL == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Missing required fields: name and url",
		})
		return
	}

	_, err := executeGitCommand("remote", "add", req.Name, req.URL)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Failed to add remote: %v", err),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Remote '%s' added successfully", req.Name),
	})
}

func handleUpdateRemote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	repoPath := r.URL.Query().Get("repoPath")

	// Use provided repoPath or fall back to config.RepoPath
	originalRepoPath := config.RepoPath
	if repoPath != "" {
		// Resolve and validate the path
		var resolvedPath string
		var err error
		if filepath.IsAbs(repoPath) {
			resolvedPath = repoPath
		} else {
			resolvedPath = filepath.Join(originalRepoPath, repoPath)
		}
		resolvedPath, err = filepath.Abs(resolvedPath)
		if err == nil {
			basePath, _ := filepath.Abs(originalRepoPath)
			if strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) || resolvedPath == basePath {
				config.RepoPath = resolvedPath
			}
		}
	}

	defer func() {
		config.RepoPath = originalRepoPath
	}()

	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Invalid request body",
		})
		return
	}

	if req.Name == "" || req.URL == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Missing required fields: name and url",
		})
		return
	}

	_, err := executeGitCommand("remote", "set-url", req.Name, req.URL)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Failed to update remote: %v", err),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Remote '%s' updated successfully", req.Name),
	})
}

func handleRemoveRemote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	repoPath := r.URL.Query().Get("repoPath")

	// Use provided repoPath or fall back to config.RepoPath
	originalRepoPath := config.RepoPath
	if repoPath != "" {
		// Resolve and validate the path
		var resolvedPath string
		var err error
		if filepath.IsAbs(repoPath) {
			resolvedPath = repoPath
		} else {
			resolvedPath = filepath.Join(originalRepoPath, repoPath)
		}
		resolvedPath, err = filepath.Abs(resolvedPath)
		if err == nil {
			basePath, _ := filepath.Abs(originalRepoPath)
			if strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) || resolvedPath == basePath {
				config.RepoPath = resolvedPath
			}
		}
	}

	defer func() {
		config.RepoPath = originalRepoPath
	}()

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Invalid request body",
		})
		return
	}

	if req.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Missing required field: name",
		})
		return
	}

	_, err := executeGitCommand("remote", "remove", req.Name)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Failed to remove remote: %v", err),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Remote '%s' removed successfully", req.Name),
	})
}

func handleSystemdStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	registered := isSystemdServiceRegistered()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"registered": registered,
	})
}

func handleSystemdRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Check if already registered
	if isSystemdServiceRegistered() {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Service is already registered with systemd",
		})
		return
	}

	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to get executable path: %v", err),
		})
		return
	}

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to get home directory: %v", err),
		})
		return
	}

	// Create systemd service directory
	serviceDir := filepath.Join(homeDir, ".config", "systemd", "user")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to create systemd directory: %v", err),
		})
		return
	}

	// Create service file content with current process arguments
	execArgs := os.Args[1:]
	var cmdLine string
	if len(execArgs) > 0 {
		cmdLine = execPath + " " + strings.Join(execArgs, " ")
	} else {
		cmdLine = execPath
	}

	// Get relevant environment variables
	envVars := []string{}
	relevantEnvVars := []string{"AIRGIT_", "SSH_", "GIT_"}
	for _, envVar := range os.Environ() {
		for _, prefix := range relevantEnvVars {
			if strings.HasPrefix(envVar, prefix) {
				envVars = append(envVars, "Environment=\""+envVar+"\"")
				break
			}
		}
	}
	environmentSection := ""
	if len(envVars) > 0 {
		environmentSection = "\n" + strings.Join(envVars, "\n")
	}

	// Create service file content
	serviceContent := fmt.Sprintf(`[Unit]
Description=AirGit - Lightweight web-based Git GUI
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal%s

[Install]
WantedBy=default.target
`, cmdLine, environmentSection)

	// Write service file
	servicePath := filepath.Join(serviceDir, "airgit.service")
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to write service file: %v", err),
		})
		return
	}

	// Reload systemd daemon
	cmd := exec.Command("systemctl", "--user", "daemon-reload")
	if err := cmd.Run(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to reload systemd daemon: %v", err),
		})
		return
	}

	// Enable service
	cmd = exec.Command("systemctl", "--user", "enable", "airgit.service")
	if err := cmd.Run(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to enable service: %v", err),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Service registered and enabled successfully",
		"path":    servicePath,
	})

	// Start the service via systemd
	go func() {
		// Wait a moment for the response to be sent before exiting
		time.Sleep(500 * time.Millisecond)

		// Start the service
		startCmd := exec.Command("systemctl", "--user", "start", "airgit.service")
		if err := startCmd.Run(); err != nil {
			log.Printf("Failed to start service after registration: %v", err)
		}

		// Exit the current process
		os.Exit(0)
	}()
}

func isSystemdServiceRegistered() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	servicePath := filepath.Join(homeDir, ".config", "systemd", "user", "airgit.service")
	_, err = os.Stat(servicePath)
	return err == nil
}

func handleSystemdServiceStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if service is registered first
	if !isSystemdServiceRegistered() {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"registered": false,
			"running":    false,
		})
		return
	}

	// Check if service is running
	cmd := exec.Command("systemctl", "--user", "is-active", "airgit")
	err := cmd.Run()

	isRunning := err == nil

	json.NewEncoder(w).Encode(map[string]interface{}{
		"registered": true,
		"running":    isRunning,
	})
}

func handleSystemdServiceStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Check if service is registered
	if !isSystemdServiceRegistered() {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Service is not registered with systemd",
		})
		return
	}

	// Check if already running
	cmd := exec.Command("systemctl", "--user", "is-active", "airgit")
	if cmd.Run() == nil {
		// Already running
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Service is already running",
		})
		return
	}

	// Start the service
	cmd = exec.Command("systemctl", "--user", "start", "airgit")
	if err := cmd.Run(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to start service: %v", err),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Service started successfully",
	})
}

func handleListCommits(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	repoPath := r.URL.Query().Get("repoPath")
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "20"
	}

	// Use provided repoPath or fall back to config.RepoPath
	originalRepoPath := config.RepoPath
	if repoPath != "" {
		// Resolve and validate the path
		var resolvedPath string
		var err error
		if filepath.IsAbs(repoPath) {
			resolvedPath = repoPath
		} else {
			resolvedPath = filepath.Join(originalRepoPath, repoPath)
		}
		resolvedPath, err = filepath.Abs(resolvedPath)
		if err == nil {
			basePath, _ := filepath.Abs(originalRepoPath)
			if strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) || resolvedPath == basePath {
				config.RepoPath = resolvedPath
			}
		}
	}

	defer func() {
		config.RepoPath = originalRepoPath
	}()

	// Get commits using git log
	format := "%H%n%an%n%ai%n%s%n---END---"
	output, err := executeGitCommand("log", "-"+limit, "--format="+format)
	if err != nil {
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Failed to get commits: %v", err),
		})
		return
	}

	commits := parseCommits(output)

	json.NewEncoder(w).Encode(Response{
		Commits: commits,
	})
}

func parseCommits(output string) []CommitInfo {
	var commits []CommitInfo
	entries := strings.Split(output, "---END---\n")

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		lines := strings.Split(entry, "\n")
		if len(lines) < 4 {
			continue
		}

		commit := CommitInfo{
			Hash:    lines[0],
			Author:  lines[1],
			Date:    lines[2],
			Message: lines[3],
		}
		commits = append(commits, commit)
	}

	return commits
}

func handleListTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	repoPath := r.URL.Query().Get("repoPath")
	
	// Use provided repoPath or fall back to config.RepoPath
	originalRepoPath := config.RepoPath
	if repoPath != "" {
		// Resolve and validate the path
		var resolvedPath string
		var err error
		if filepath.IsAbs(repoPath) {
			resolvedPath = repoPath
		} else {
			resolvedPath = filepath.Join(originalRepoPath, repoPath)
		}
		resolvedPath, err = filepath.Abs(resolvedPath)
		if err == nil {
			basePath, _ := filepath.Abs(originalRepoPath)
			if strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) || resolvedPath == basePath {
				config.RepoPath = resolvedPath
			}
		}
	}

	defer func() {
		config.RepoPath = originalRepoPath
	}()

	output, err := executeGitCommand("tag", "-l")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Failed to list tags: %v", err),
		})
		return
	}

	var tags []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			tags = append(tags, line)
		}
	}

	json.NewEncoder(w).Encode(Response{
		Tags: tags,
	})
}

func handleCreateTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		TagName string `json:"tagName"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Invalid request body",
		})
		return
	}

	if req.TagName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Tag name is required",
		})
		return
	}

	var logs []string
	var args []string
	var output string
	var err error

	if req.Message != "" {
		args = []string{"tag", "-a", req.TagName, "-m", req.Message}
		logs = append(logs, fmt.Sprintf("$ git tag -a %s -m \"%s\"", req.TagName, req.Message))
	} else {
		args = []string{"tag", req.TagName}
		logs = append(logs, fmt.Sprintf("$ git tag %s", req.TagName))
	}

	output, err = executeGitCommand(args...)
	if output != "" {
		logs = append(logs, output)
	}
	if err != nil {
		resp := Response{
			Error: fmt.Sprintf("Failed to create tag: %v", err),
			Log:   logs,
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	logs = append(logs, fmt.Sprintf("✓ Tag '%s' created!", req.TagName))

	json.NewEncoder(w).Encode(Response{
		Commit: req.TagName,
		Log:    logs,
	})
}

func handlePushTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	repoPath := r.URL.Query().Get("repoPath")
	remote := r.URL.Query().Get("remote")
	if remote == "" {
		remote = "origin"
	}
	
	// Use provided repoPath or fall back to config.RepoPath
	originalRepoPath := config.RepoPath
	if repoPath != "" {
		// Resolve and validate the path
		var resolvedPath string
		var err error
		if filepath.IsAbs(repoPath) {
			resolvedPath = repoPath
		} else {
			resolvedPath = filepath.Join(originalRepoPath, repoPath)
		}
		resolvedPath, err = filepath.Abs(resolvedPath)
		if err == nil {
			basePath, _ := filepath.Abs(originalRepoPath)
			if strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) || resolvedPath == basePath {
				config.RepoPath = resolvedPath
			}
		}
	}

	defer func() {
		config.RepoPath = originalRepoPath
	}()

	var req struct {
		TagName string `json:"tagName"`
		All     bool   `json:"all"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Error: "Invalid request body",
		})
		return
	}

	var logs []string
	var output string
	var err error

	if req.All {
		output, err = executeGitCommand("push", remote, "--tags")
		logs = append(logs, fmt.Sprintf("$ git push %s --tags", remote))
	} else {
		if req.TagName == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(Response{
				Error: "Tag name is required",
			})
			return
		}
		output, err = executeGitCommand("push", remote, req.TagName)
		logs = append(logs, fmt.Sprintf("$ git push %s %s", remote, req.TagName))
	}

	if output != "" {
		logs = append(logs, output)
	}
	if err != nil {
		if strings.Contains(output, "up to date") || strings.Contains(output, "up-to-date") {
			logs = append(logs, "✓ Everything is already up to date!")
		} else {
			resp := Response{
				Error: fmt.Sprintf("git push failed: %v", err),
				Log:   logs,
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(resp)
			return
		}
	} else {
		logs = append(logs, "✓ Push successful!")
	}

	json.NewEncoder(w).Encode(Response{
		Log: logs,
	})
}

func handleListGitHubIssues(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	repoPath := r.URL.Query().Get("repoPath")
	
	// Use provided repoPath or fall back to config.RepoPath
	originalRepoPath := config.RepoPath
	if repoPath != "" {
		// Resolve and validate the path
		var resolvedPath string
		var err error
		if filepath.IsAbs(repoPath) {
			resolvedPath = repoPath
		} else {
			resolvedPath = filepath.Join(originalRepoPath, repoPath)
		}
		resolvedPath, err = filepath.Abs(resolvedPath)
		if err == nil {
			basePath, _ := filepath.Abs(originalRepoPath)
			if strings.HasPrefix(resolvedPath, basePath+string(filepath.Separator)) || resolvedPath == basePath {
				config.RepoPath = resolvedPath
			}
		}
	}

	defer func() {
		config.RepoPath = originalRepoPath
	}()

	// Get GitHub remote URL
	output, err := executeCommand("git", "config", "--get", "remote.origin.url")
	if err != nil || strings.TrimSpace(output) == "" {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "No GitHub remote found. Make sure 'origin' remote is configured.",
		})
		return
	}

	remoteURL := strings.TrimSpace(output)

	// Parse GitHub URL to extract owner/repo
	owner, repo := parseGitHubURL(remoteURL)
	if owner == "" || repo == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Could not parse GitHub repository from remote URL: " + remoteURL,
		})
		return
	}

	// Fetch issues using gh CLI
	cmd := exec.Command("gh", "issue", "list", "--json", "number,title,body,author,assignees", "-L", "50")
	cmd.Dir = config.RepoPath
	cmd.Env = append(os.Environ(), "GITHUB_TOKEN="+os.Getenv("GITHUB_TOKEN"))
	
	var issuesOutput bytes.Buffer
	var issuesError bytes.Buffer
	cmd.Stdout = &issuesOutput
	cmd.Stderr = &issuesError
	
	log.Printf("gh command: cwd=%s, owner=%s, repo=%s", config.RepoPath, owner, repo)
	
	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(issuesError.String())
		log.Printf("gh issue list error: %v, stderr: %s", err, errMsg)
		
		// Return error message to UI
		json.NewEncoder(w).Encode(map[string]interface{}{
			"owner":     owner,
			"repo":      repo,
			"remoteUrl": remoteURL,
			"issues":    []interface{}{},
			"error":     errMsg,
		})
		return
	}

	// Parse JSON output
	var issues []map[string]interface{}
	outputStr := strings.TrimSpace(issuesOutput.String())
	log.Printf("gh output: %s", outputStr)
	
	if outputStr == "" {
		// No issues found
		issues = []map[string]interface{}{}
	} else if err := json.Unmarshal(issuesOutput.Bytes(), &issues); err != nil {
		log.Printf("parse issues error: %v, output: %s", err, outputStr)
		issues = []map[string]interface{}{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"owner":     owner,
		"repo":      repo,
		"remoteUrl": remoteURL,
		"issues":    issues,
	})
}

func parseGitHubURL(remoteURL string) (owner, repo string) {
	// Handle both HTTPS and SSH URLs
	remoteURL = strings.TrimSpace(remoteURL)

	// SSH: git@github.com:owner/repo.git
	if strings.HasPrefix(remoteURL, "git@github.com:") {
		parts := strings.TrimPrefix(remoteURL, "git@github.com:")
		parts = strings.TrimSuffix(parts, ".git")
		elements := strings.Split(parts, "/")
		if len(elements) >= 2 {
			return elements[0], elements[1]
		}
	}

	// HTTPS: https://github.com/owner/repo.git
	if strings.Contains(remoteURL, "github.com") {
		parts := strings.Split(remoteURL, "/")
		if len(parts) >= 2 {
			repo = strings.TrimSuffix(parts[len(parts)-1], ".git")
			owner = parts[len(parts)-2]
			if owner != "" && repo != "" {
				return owner, repo
			}
		}
	}

	return "", ""
}

func executeCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = config.RepoPath

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()

	result := strings.TrimSpace(output.String())

	if err != nil {
		return result, err
	}

	return result, nil
}

func handleAgentTrigger(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	log.Printf("handleAgentTrigger called: method=%s", r.Method)

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "POST only"})
		return
	}

	var payload struct {
		IssueNumber int    `json:"issue_number"`
		IssueTitle  string `json:"issue_title"`
		IssueBody   string `json:"issue_body"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("handleAgentTrigger: decode error: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	log.Printf("handleAgentTrigger: Issue #%d - %s", payload.IssueNumber, payload.IssueTitle)

	go func() {
		log.Printf("Agent goroutine started for issue #%d", payload.IssueNumber)
		processAgentIssue(payload.IssueNumber, payload.IssueTitle, payload.IssueBody)
	}()

	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func processAgentIssue(issueNumber int, issueTitle, issueBody string) {
	log.Printf("processAgentIssue: starting for #%d", issueNumber)
	
	// Update status to running
	agentStatusMutex.Lock()
	agentStatus[issueNumber] = AgentStatus{
		IssueNumber: issueNumber,
		Status:      "running",
		Message:     "Processing issue",
		StartTime:   time.Now(),
	}
	agentStatusMutex.Unlock()
	
	timestamp := time.Now().UnixNano() / 1000000
	branchName := fmt.Sprintf("airgit/issue-%d-%d", issueNumber, timestamp)
	worktreePath := filepath.Join("/tmp/airgit", fmt.Sprintf("issue-%d-%d", issueNumber, timestamp))
	log.Printf("processAgentIssue: branch=%s, worktreePath=%s", branchName, worktreePath)
	
	// Ensure /tmp/airgit exists
	if err := os.MkdirAll("/tmp/airgit", 0755); err != nil {
		log.Printf("Failed to create /tmp/airgit: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to create worktree directory: %v", err),
			StartTime:   time.Now().Add(-1 * time.Minute),
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}
	
	gitCmd := func(args ...string) error {
		log.Printf("git: %v", args)
		cmd := exec.Command("git", args...)
		cmd.Dir = config.RepoPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("git error: %v, output: %s", err, string(out))
		} else {
			log.Printf("git ok: %s", string(out))
		}
		return err
	}

	gitCmd("fetch", "origin")
	
	// Create git worktree
	log.Printf("Creating git worktree at %s", worktreePath)
	if err := gitCmd("worktree", "add", worktreePath, "-b", branchName, "main"); err != nil {
		log.Printf("worktree creation failed: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to create worktree: %v", err),
			StartTime:   time.Now().Add(-1 * time.Minute),
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}
	defer func() {
		log.Printf("Removing git worktree at %s", worktreePath)
		gitCmd("worktree", "remove", worktreePath)
	}()

	// Create prompt for Copilot CLI
	prompt := fmt.Sprintf(`You are a code implementation assistant. Please analyze and implement the solution for GitHub issue #%d.

Issue Title: %s

Issue Description:
%s

Instructions:
1. Analyze the issue and understand what needs to be implemented
2. Write clean, production-ready code that solves this issue
3. Follow the existing code style and conventions in the repository
4. Add appropriate tests if needed
5. Update any relevant documentation
6. Make sure all changes are properly committed

Please implement the complete solution.`, issueNumber, issueTitle, issueBody)

	log.Printf("Creating implementation file and commit...")
	
	// Create a marker file to track the issue
	markerContent := fmt.Sprintf(`Issue #%d Implementation
Title: %s
Status: In Progress

The Copilot CLI will implement the solution based on the issue description.
`, issueNumber, issueTitle)
	
	markerFile := filepath.Join(config.RepoPath, fmt.Sprintf(".issue-%d-marker", issueNumber))
	if err := os.WriteFile(markerFile, []byte(markerContent), 0644); err != nil {
		log.Printf("Failed to write marker file: %v", err)
	}
	defer os.Remove(markerFile)
	
	// Call Copilot CLI to implement the solution
	log.Printf("Invoking Copilot CLI with /delegate command for issue #%d", issueNumber)
	
	// Use copilot /delegate to generate implementation
	copilotCmd := exec.Command("copilot", "/delegate", "--prompt", prompt)
	copilotCmd.Dir = worktreePath
	copilotCmd.Env = os.Environ()
	
	var copilotOut bytes.Buffer
	var copilotErr bytes.Buffer
	copilotCmd.Stdout = &copilotOut
	copilotCmd.Stderr = &copilotErr
	
	err := copilotCmd.Run()
	copilotOutput := copilotOut.String()
	copilotError := copilotErr.String()
	
	log.Printf("Copilot output: %s", copilotOutput)
	log.Printf("Copilot error: %s", copilotError)
	
	if err != nil {
		log.Printf("Copilot CLI error: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Copilot implementation failed: %v", err),
			StartTime:   time.Now().Add(-1 * time.Minute),
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	// Create PR using GitHub API
	prNumber, prURL, err := createPullRequest(issueNumber, issueTitle, branchName)
	if err != nil {
		log.Printf("Failed to create PR: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to create PR: %v", err),
			StartTime:   time.Now().Add(-1 * time.Minute),
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	log.Printf("PR created: #%d at %s", prNumber, prURL)
	agentStatusMutex.Lock()
	agentStatus[issueNumber] = AgentStatus{
		IssueNumber: issueNumber,
		Status:      "completed",
		Message:     fmt.Sprintf("PR created: %s", prURL),
		StartTime:   time.Now().Add(-2 * time.Minute),
		EndTime:     time.Now(),
		PRNumber:    prNumber,
	}
	agentStatusMutex.Unlock()
}

func createPullRequest(issueNumber int, issueTitle string, branchName string) (int, string, error) {
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		return 0, "", fmt.Errorf("GH_TOKEN not set")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Get the remote URL to extract owner and repo
	ownerRepo, err := getGitHubOwnerRepo(config.RepoPath)
	if err != nil {
		return 0, "", err
	}

	parts := strings.Split(ownerRepo, "/")
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid owner/repo format: %s", ownerRepo)
	}

	owner, repo := parts[0], parts[1]

	prTitle := fmt.Sprintf("Issue #%d: %s", issueNumber, issueTitle)
	prBody := fmt.Sprintf("Fixes #%d\n\nAuto-generated implementation for issue #%d", issueNumber, issueNumber)

	newPR := &github.NewPullRequest{
		Title:               github.String(prTitle),
		Head:                github.String(branchName),
		Base:                github.String("main"),
		Body:                github.String(prBody),
		MaintainerCanModify: github.Bool(true),
	}

	pr, _, err := client.PullRequests.Create(ctx, owner, repo, newPR)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create PR: %w", err)
	}

	return pr.GetNumber(), pr.GetHTMLURL(), nil
}

func getGitHubOwnerRepo(repoPath string) (string, error) {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	url := strings.TrimSpace(string(out))
	// Handle both https and git@ URLs
	if strings.Contains(url, "github.com") {
		if strings.HasPrefix(url, "git@") {
			// git@github.com:owner/repo.git
			parts := strings.Split(url, ":")
			if len(parts) == 2 {
				repo := parts[1]
				repo = strings.TrimSuffix(repo, ".git")
				return repo, nil
			}
		} else {
			// https://github.com/owner/repo.git
			parts := strings.Split(url, "/")
			if len(parts) >= 2 {
				repo := strings.TrimSuffix(parts[len(parts)-1], ".git")
				owner := parts[len(parts)-2]
				return fmt.Sprintf("%s/%s", owner, repo), nil
			}
		}
	}

	return "", fmt.Errorf("could not extract owner/repo from URL: %s", url)
}

func handleAgentProcess(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "application/json")

if r.Method != http.MethodPost {
w.WriteHeader(http.StatusMethodNotAllowed)
json.NewEncoder(w).Encode(map[string]interface{}{"error": "POST only"})
return
}

var payload struct {
IssueNumber int    `json:"issue_number"`
IssueTitle  string `json:"issue_title"`
IssueBody   string `json:"issue_body"`
}

if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
w.WriteHeader(http.StatusBadRequest)
json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
return
}

log.Printf("Agent process started: Issue #%d - %s", payload.IssueNumber, payload.IssueTitle)

go func() {
log.Printf("Agent goroutine started for issue #%d", payload.IssueNumber)
processAgentIssue(payload.IssueNumber, payload.IssueTitle, payload.IssueBody)
}()

json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Agent processing started"})
}

func handleAgentStatus(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "application/json")

if r.Method != http.MethodGet {
w.WriteHeader(http.StatusMethodNotAllowed)
json.NewEncoder(w).Encode(map[string]interface{}{"error": "GET only"})
return
}

issueNumberStr := r.URL.Query().Get("issue_number")
if issueNumberStr == "" {
w.WriteHeader(http.StatusBadRequest)
json.NewEncoder(w).Encode(map[string]interface{}{"error": "Missing issue_number"})
return
}

issueNumber, err := strconv.Atoi(issueNumberStr)
if err != nil {
w.WriteHeader(http.StatusBadRequest)
json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid issue_number"})
return
}

agentStatusMutex.Lock()
status, exists := agentStatus[issueNumber]
agentStatusMutex.Unlock()

if !exists {
w.WriteHeader(http.StatusNotFound)
json.NewEncoder(w).Encode(map[string]interface{}{"error": "No status for this issue"})
return
}

w.WriteHeader(http.StatusOK)
json.NewEncoder(w).Encode(status)
}
