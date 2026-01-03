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
	Branch   string       `json:"branch,omitempty"`
	RepoName string       `json:"repoName,omitempty"`
	Error    string       `json:"error,omitempty"`
	Log      []string     `json:"log,omitempty"`
	Commit   string       `json:"commit,omitempty"`
	Branches []string     `json:"branches,omitempty"`
	Tags     []string     `json:"tags,omitempty"`
	Remotes  []string     `json:"remotes,omitempty"`
	Ahead    int          `json:"ahead,omitempty"`
	Behind   int          `json:"behind,omitempty"`
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
	PRNumber    int       `json:"prNumber,omitempty"`
	PRURL       string    `json:"prUrl,omitempty"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime,omitempty"`
}

var config Config
var baseRepoPath string
var agentStatus map[int]AgentStatus // issueNumber -> status
var agentStatusMutex sync.Mutex
var githubAuthProcess *exec.Cmd // Track ongoing GitHub auth process
var githubAuthMutex sync.Mutex

func init() {
	// Determine default RepoPath
	defaultRepoPath := os.Getenv("HOME")

	// If current directory is a git repository, use it as default
	if cwd, err := os.Getwd(); err == nil {
		if isGitRepo(cwd) {
			defaultRepoPath = cwd
		}
	}

	config = Config{
		RepoPath:   getEnv("AIRGIT_REPO_PATH", defaultRepoPath),
		ListenAddr: getEnv("AIRGIT_LISTEN_ADDR", "0.0.0.0"),
		ListenPort: getEnv("AIRGIT_LISTEN_PORT", "8080"),
		TLSCert:    getEnv("AIRGIT_TLS_CERT", ""),
		TLSKey:     getEnv("AIRGIT_TLS_KEY", ""),
	}
	baseRepoPath = config.RepoPath
	agentStatus = make(map[int]AgentStatus)

	log.Printf("Config: RepoPath=%s", config.RepoPath)
}

func isGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	if info, err := os.Stat(gitPath); err == nil {
		// .git can be either a directory or a file (for worktrees)
		return info.IsDir() || info.Mode().IsRegular()
	}
	return false
}

func getEnv(key, defaultVal string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultVal
}

// resolveAndValidateRepoPath resolves a repository path and validates it's within the base path and is a git repo
func resolveAndValidateRepoPath(repoPath, basePath string) (string, bool) {
	if repoPath == "" {
		return "", false
	}

	var resolvedPath string
	if filepath.IsAbs(repoPath) {
		resolvedPath = repoPath
	} else {
		resolvedPath = filepath.Join(basePath, repoPath)
	}

	var err error
	resolvedPath, err = filepath.Abs(resolvedPath)
	if err != nil {
		return "", false
	}

	basePathAbs, _ := filepath.Abs(basePath)
	// Check if resolved path is within base path
	if !strings.HasPrefix(resolvedPath, basePathAbs+string(filepath.Separator)) && resolvedPath != basePathAbs {
		return "", false
	}

	// Check if it's a valid git repository
	if !isGitRepo(resolvedPath) {
		return "", false
	}

	return resolvedPath, true
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
	http.HandleFunc("/api/systemd/rebuild-restart", handleSystemdRebuildRestart)
	http.HandleFunc("/api/github/issues", handleListGitHubIssues)
	http.HandleFunc("/api/github/issues/create", handleCreateGitHubIssue)
	http.HandleFunc("/api/github/auth/status", handleGitHubAuthStatus)
	http.HandleFunc("/api/github/auth/login", handleGitHubAuthLogin)
	http.HandleFunc("/api/github/prs", handleListGitHubPRs)
	http.HandleFunc("/api/github/pr/reviews", handleGetPRReviews)
	http.HandleFunc("/api/agent/trigger", handleAgentTrigger)
	http.HandleFunc("/api/agent/process", handleAgentProcess)
	http.HandleFunc("/api/agent/status", handleAgentStatus)
	http.HandleFunc("/api/agent/apply-review", handleAgentApplyReview)
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
		if validPath, ok := resolveAndValidateRepoPath(repoPath, originalRepoPath); ok {
			config.RepoPath = validPath
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
		} else if strings.Contains(output, "rejected") && strings.Contains(output, "non-fast-forward") {
			// Remote has changes that we don't have - need to pull first
			logs = append(logs, "⚠ Push rejected: remote has changes")
			logs = append(logs, "Attempting to pull and merge...")
			
			// Try to pull with merge
			pullOutput, pullErr := executeGitCommand("pull", remote, branch)
			logs = append(logs, fmt.Sprintf("$ git pull %s %s", remote, branch))
			if pullOutput != "" {
				logs = append(logs, pullOutput)
			}
			
			if pullErr != nil {
				if strings.Contains(pullOutput, "CONFLICT") {
					// Conflict detected during pull
					conflictFiles := getConflictFiles()
					resp := Response{
						Error: fmt.Sprintf("Merge conflict detected in %d file(s)", len(conflictFiles)),
						Log:   append(logs, "Please resolve conflicts manually"),
					}
					w.WriteHeader(http.StatusConflict)
					json.NewEncoder(w).Encode(resp)
					return
				}
				resp := Response{
					Error: fmt.Sprintf("Pull failed: %v", pullErr),
					Log:   logs,
				}
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(resp)
				return
			}
			
			logs = append(logs, "✓ Pull successful, retrying push...")
			
			// Retry push after successful pull
			retryOutput, retryErr := executeGitCommand("push", remote, branch)
			logs = append(logs, fmt.Sprintf("$ git push %s %s", remote, branch))
			if retryOutput != "" {
				logs = append(logs, retryOutput)
			}
			if retryErr != nil {
				resp := Response{
					Error: fmt.Sprintf("Push failed after pull: %v", retryErr),
					Log:   logs,
				}
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(resp)
				return
			}
			logs = append(logs, "✓ Push successful after pull!")
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
		// Check for merge conflicts
		if strings.Contains(output, "CONFLICT") {
			conflictFiles := getConflictFiles()
			logs = append(logs, fmt.Sprintf("⚠ Merge conflict detected in %d file(s)", len(conflictFiles)))
			logs = append(logs, "Attempting automatic conflict resolution...")
			
			// Try to resolve conflicts automatically
			resolved, resolveErr := autoResolveConflicts(conflictFiles)
			if resolveErr != nil {
				logs = append(logs, fmt.Sprintf("✗ Automatic resolution failed: %v", resolveErr))
				logs = append(logs, "Files with conflicts:")
				for _, file := range conflictFiles {
					logs = append(logs, fmt.Sprintf("  - %s", file))
				}
				resp := Response{
					Error: "Merge conflict requires manual resolution",
					Log:   logs,
				}
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(resp)
				return
			}
			
			if len(resolved) > 0 {
				logs = append(logs, fmt.Sprintf("✓ Auto-resolved %d file(s):", len(resolved)))
				for _, file := range resolved {
					logs = append(logs, fmt.Sprintf("  - %s", file))
				}
				
				// Commit the resolved changes
				commitMsg := fmt.Sprintf("Merge %s/%s with auto-resolved conflicts", remote, branch)
				commitOutput, commitErr := executeGitCommand("commit", "-m", commitMsg)
				logs = append(logs, "$ git commit -m \""+commitMsg+"\"")
				if commitOutput != "" {
					logs = append(logs, commitOutput)
				}
				if commitErr != nil {
					resp := Response{
						Error: fmt.Sprintf("Failed to commit resolved conflicts: %v", commitErr),
						Log:   logs,
					}
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(resp)
					return
				}
				logs = append(logs, "✓ Pull successful with auto-resolved conflicts!")
			} else {
				logs = append(logs, "No conflicts could be auto-resolved")
				resp := Response{
					Error: "Merge conflict requires manual resolution",
					Log:   logs,
				}
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(resp)
				return
			}
		} else {
			resp := Response{
				Error: fmt.Sprintf("git pull failed: %v", err),
				Log:   logs,
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(resp)
			return
		}
	} else {
		logs = append(logs, "✓ Pull successful!")
	}

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

// getConflictFiles returns a list of files with merge conflicts
func getConflictFiles() []string {
	output, err := executeGitCommand("diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return []string{}
	}
	
	var files []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

// autoResolveConflicts attempts to automatically resolve merge conflicts
// Returns the list of successfully resolved files
func autoResolveConflicts(conflictFiles []string) ([]string, error) {
	var resolved []string
	
	for _, file := range conflictFiles {
		filePath := filepath.Join(config.RepoPath, file)
		
		// Read the conflicted file
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		
		// Check if the conflict can be auto-resolved
		// Simple strategy: if one side is empty or only whitespace, use the other side
		lines := strings.Split(string(content), "\n")
		var newContent strings.Builder
		inConflict := false
		oursContent := ""
		theirsContent := ""
		conflictStart := -1
		
		for i, line := range lines {
			if strings.HasPrefix(line, "<<<<<<<") {
				inConflict = true
				conflictStart = i
				oursContent = ""
				theirsContent = ""
			} else if strings.HasPrefix(line, "=======") && inConflict {
				// Switch from ours to theirs
			} else if strings.HasPrefix(line, ">>>>>>>") && inConflict {
				inConflict = false
				
				// Try to resolve
				oursEmpty := strings.TrimSpace(oursContent) == ""
				theirsEmpty := strings.TrimSpace(theirsContent) == ""
				
				if oursEmpty && !theirsEmpty {
					// Use theirs
					newContent.WriteString(theirsContent)
				} else if theirsEmpty && !oursEmpty {
					// Use ours
					newContent.WriteString(oursContent)
				} else if oursContent == theirsContent {
					// Both sides are the same
					newContent.WriteString(oursContent)
				} else {
					// Cannot auto-resolve, restore conflict markers
					newContent.WriteString("<<<<<<<\n")
					newContent.WriteString(oursContent)
					newContent.WriteString("=======\n")
					newContent.WriteString(theirsContent)
					newContent.WriteString(">>>>>>>\n")
				}
			} else if inConflict {
				// Collect content
				if oursContent != "" || !strings.HasPrefix(line, "=======") {
					if strings.HasPrefix(strings.Join(lines[conflictStart:i], "\n"), "<<<<<<<") && !strings.Contains(strings.Join(lines[conflictStart:i], "\n"), "=======") {
						oursContent += line + "\n"
					} else {
						theirsContent += line + "\n"
					}
				}
			} else {
				newContent.WriteString(line + "\n")
			}
		}
		
		// Check if all conflicts were resolved
		finalContent := newContent.String()
		if !strings.Contains(finalContent, "<<<<<<<") && !strings.Contains(finalContent, "=======") && !strings.Contains(finalContent, ">>>>>>>") {
			// Write the resolved content
			if err := os.WriteFile(filePath, []byte(finalContent), 0644); err != nil {
				continue
			}
			
			// Stage the resolved file
			if _, err := executeGitCommand("add", file); err != nil {
				continue
			}
			
			resolved = append(resolved, file)
		}
	}
	
	if len(resolved) == 0 && len(conflictFiles) > 0 {
		return resolved, fmt.Errorf("no conflicts could be automatically resolved")
	}
	
	return resolved, nil
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
				"error":  fmt.Sprintf("Failed to checkout branch '%s': %v. Output: %s", branch, err, output),
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
		RepoName string `json:"repoName"`
		Subdirs  string `json:"subdirs"`
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

func handleSystemdRebuildRestart(w http.ResponseWriter, r *http.Request) {
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

	// Get the directory where the binary is located
	exePath, err := os.Executable()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to determine executable path: %v", err),
		})
		return
	}
	workDir := filepath.Dir(exePath)

	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", "airgit", ".")
	buildCmd.Dir = workDir

	var buildOutput bytes.Buffer
	buildCmd.Stderr = &buildOutput
	buildCmd.Stdout = &buildOutput

	if err := buildCmd.Run(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Build failed: %v", err),
			"details": buildOutput.String(),
		})
		return
	}

	// Send success response before restart
	// This ensures the client receives the response before the service terminates
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Build successful, restarting service...",
	})

	// Flush the response to ensure it's sent
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Schedule restart after a short delay to allow response to be sent
	go func() {
		time.Sleep(500 * time.Millisecond)
		restartCmd := exec.Command("systemctl", "--user", "restart", "airgit")
		var restartOutput bytes.Buffer
		restartCmd.Stderr = &restartOutput
		restartCmd.Stdout = &restartOutput
		if err := restartCmd.Run(); err != nil {
			log.Printf("Failed to restart service: %v, output: %s", err, restartOutput.String())
		} else {
			log.Printf("Service restart initiated successfully")
		}
	}()
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

func handleCreateGitHubIssue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "POST only",
		})
		return
	}

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
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		Labels []string `json:"labels"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid request body",
		})
		return
	}

	if req.Title == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Title is required",
		})
		return
	}

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

	// Build gh CLI command arguments
	args := []string{"issue", "create", "--title", req.Title}

	if req.Body != "" {
		args = append(args, "--body", req.Body)
	}

	for _, label := range req.Labels {
		args = append(args, "--label", label)
	}

	// Create issue using gh CLI
	cmd := exec.Command("gh", args...)
	cmd.Dir = config.RepoPath
	cmd.Env = os.Environ()

	var issueOutput bytes.Buffer
	var issueError bytes.Buffer
	cmd.Stdout = &issueOutput
	cmd.Stderr = &issueError

	log.Printf("Creating GitHub issue: title=%s, owner=%s, repo=%s", req.Title, owner, repo)

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(issueError.String())
		log.Printf("gh issue create error: %v, stderr: %s", err, errMsg)

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to create issue",
			"details": errMsg,
		})
		return
	}

	issueURL := strings.TrimSpace(issueOutput.String())
	log.Printf("Issue created: %s", issueURL)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"url":     issueURL,
		"message": "Issue created successfully",
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

func handleGitHubAuthStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "GET only"})
		return
	}

	// Check if gh copilot is actually usable (requires OAuth, not just GH_TOKEN)
	// Test with new copilot CLI
	// IMPORTANT: unset GH_TOKEN to avoid it being used instead of OAuth token
	homeDir, _ := os.UserHomeDir()
	copilotPath := filepath.Join(homeDir, "bin", "copilot")
	if _, err := os.Stat(copilotPath); os.IsNotExist(err) {
		copilotPath = "/usr/local/bin/copilot"
	}

	cmd := exec.Command("bash", "-c", fmt.Sprintf(`unset GH_TOKEN && echo "test" | timeout 5 %s --allow-all-tools 2>&1`, copilotPath))
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	cmd.Run()

	output := out.String()

	// Check for various error conditions
	hasOAuthError := strings.Contains(output, "OAuth") || strings.Contains(output, "gh auth login") || strings.Contains(output, "not authenticated")
	hasInternalError := strings.Contains(output, "internal server error")
	hasScopeError := strings.Contains(output, "403") || strings.Contains(output, "forbidden")
	hasNotFoundError := strings.Contains(output, "command not found") || strings.Contains(output, "No such file")

	var isAuthenticated bool

	// If copilot CLI not found, definitely not authenticated
	if hasNotFoundError {
		isAuthenticated = false
	} else {
		// If there's an OAuth error, not authenticated
		// But internal server error might mean authenticated with wrong subscription
		isAuthenticated = !hasOAuthError && !hasScopeError
	}

	// Also check gh auth status for additional info
	authCmd := exec.Command("bash", "-c", `unset GH_TOKEN && gh auth status 2>&1`)
	var authOut bytes.Buffer
	authCmd.Stdout = &authOut
	authCmd.Stderr = &authOut
	authCmd.Run()

	statusOutput := authOut.String()

	// If gh auth status succeeds but copilot fails with internal error, check token scopes
	if strings.Contains(statusOutput, "Logged in") && hasInternalError {
		// Check if copilot scope is present
		hasCopilotScope := strings.Contains(statusOutput, "copilot")
		isAuthenticated = hasCopilotScope
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated":  isAuthenticated,
		"status":         statusOutput,
		"copilot_output": output,
	})
}

func handleGitHubAuthLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "POST only"})
		return
	}

	log.Printf("handleGitHubAuthLogin: Starting GitHub device flow authentication")

	githubAuthMutex.Lock()
	defer githubAuthMutex.Unlock()

	// Kill any existing auth process
	if githubAuthProcess != nil {
		log.Printf("Killing existing auth process")
		githubAuthProcess.Process.Kill()
		githubAuthProcess = nil
	}

	// Start gh auth login and capture initial output for device code
	cmd := exec.Command("bash", "-c", `unset GH_TOKEN && gh auth login -s copilot 2>&1`)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Failed to create stdout pipe: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to start authentication",
			"success": false,
		})
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("Failed to create stderr pipe: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to start authentication",
			"success": false,
		})
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start gh auth login: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to start authentication",
			"success": false,
		})
		return
	}

	// Read initial output to get device code
	var output bytes.Buffer
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				output.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				output.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	// Wait for device code to appear (max 5 seconds)
	time.Sleep(3 * time.Second)

	outputStr := output.String()
	log.Printf("gh auth login initial output: %s", outputStr)

	// Parse the one-time code and URL from output
	var code, url string
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "one-time code:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				code = strings.TrimSpace(parts[len(parts)-1])
			}
		}
		if strings.Contains(line, "https://github.com/login/device") {
			if idx := strings.Index(line, "https://github.com/login/device"); idx >= 0 {
				url = "https://github.com/login/device"
			}
		}
	}

	if code == "" || url == "" {
		log.Printf("Failed to parse device code from output. code='%s', url='%s'", code, url)
		cmd.Process.Kill()
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to start device flow authentication",
			"success": false,
			"output":  outputStr,
		})
		return
	}

	// Store the process so it continues running in background
	githubAuthProcess = cmd

	// Start goroutine to wait for completion
	go func() {
		cmd.Wait()
		githubAuthMutex.Lock()
		if githubAuthProcess == cmd {
			githubAuthProcess = nil
		}
		githubAuthMutex.Unlock()
		log.Printf("GitHub auth process completed")
	}()

	log.Printf("Device flow started: code=%s, url=%s", code, url)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"device_flow": true,
		"code":        code,
		"url":         url,
		"message":     "Please authenticate using the provided code",
	})
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

// stripAnsiCodes removes ANSI escape sequences from a string
func stripAnsiCodes(str string) string {
	// Simple regex-free approach: remove ESC sequences
	var result strings.Builder
	inEscape := false
	for i := 0; i < len(str); i++ {
		if str[i] == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			// Skip until we find a letter (end of escape sequence)
			if (str[i] >= 'A' && str[i] <= 'Z') || (str[i] >= 'a' && str[i] <= 'z') {
				inEscape = false
			}
			continue
		}
		result.WriteByte(str[i])
	}
	return result.String()
}

// extractMeaningfulProgress extracts useful progress messages from Copilot CLI output
func extractMeaningfulProgress(line string) (string, bool) {
	line = stripAnsiCodes(line)
	line = strings.TrimSpace(line)

	// Ignore empty lines
	if len(line) == 0 {
		return "", false
	}

	// Ignore lines that are just control characters or very short
	if len(line) < 3 {
		return "", false
	}

	// Look for meaningful patterns from Copilot CLI output
	meaningfulPrefixes := []string{
		"Analyzing",
		"Reading",
		"Processing",
		"Generating",
		"Creating",
		"Editing",
		"Writing",
		"Updating",
		"Searching",
		"Found",
		"Suggesting",
		"Applying",
		"Running",
		"Executing",
		"Checking",
		"Validating",
		"Building",
		"Testing",
		"✓",
		"✗",
		"●",
		"○",
		"→",
	}

	for _, prefix := range meaningfulPrefixes {
		if strings.HasPrefix(line, prefix) || strings.Contains(line, prefix) {
			// Truncate if too long
			if len(line) > 120 {
				return line[:117] + "...", true
			}
			return line, true
		}
	}

	// If line contains question marks or ends with colon, it might be prompting
	if strings.Contains(line, "?") || strings.HasSuffix(line, ":") {
		if len(line) > 120 {
			return line[:117] + "...", true
		}
		return line, true
	}

	return "", false
}

func processAgentIssue(issueNumber int, issueTitle, issueBody string) {
	log.Printf("processAgentIssue: starting for #%d", issueNumber)

	startTime := time.Now()

	// Update status to running
	agentStatusMutex.Lock()
	agentStatus[issueNumber] = AgentStatus{
		IssueNumber: issueNumber,
		Status:      "running",
		Message:     "Processing issue",
		StartTime:   startTime,
	}
	agentStatusMutex.Unlock()

	timestamp := time.Now().UnixNano() / 1000000
	branchName := fmt.Sprintf("airgit/issue-%d-%d", issueNumber, timestamp)
	worktreeBasePath := filepath.Join("/var/tmp/vibe-kanban/worktrees", fmt.Sprintf("%04x-issue-%d-%d", timestamp&0xFFFF, issueNumber, timestamp))
	// worktree is created directly at worktreeBasePath, no AirGit subdirectory
	worktreePath := worktreeBasePath
	log.Printf("processAgentIssue: branch=%s, worktreePath=%s", branchName, worktreePath)

	// Get the main repository path (not worktree)
	repoPath := config.RepoPath

	// Check if current path is a worktree and get the main repo
	gitDirFile := filepath.Join(repoPath, ".git")
	if data, err := os.ReadFile(gitDirFile); err == nil {
		// This is a worktree, extract main repo path
		gitdir := strings.TrimSpace(string(data))
		if strings.HasPrefix(gitdir, "gitdir: ") {
			gitdir = strings.TrimPrefix(gitdir, "gitdir: ")
			// gitdir points to .git/worktrees/XXX
			// Main repo is at the parent of .git
			mainGitDir := filepath.Dir(filepath.Dir(gitdir))
			repoPath = filepath.Dir(mainGitDir)
			log.Printf("Detected worktree, using main repo: %s", repoPath)
		}
	}

	log.Printf("Using repository path: %s", repoPath)

	// Helper function to update status message
	updateProgress := func(message string) {
		agentStatusMutex.Lock()
		if status, ok := agentStatus[issueNumber]; ok {
			status.Message = message
			agentStatus[issueNumber] = status
		}
		agentStatusMutex.Unlock()
		log.Printf("Agent progress #%d: %s", issueNumber, message)
	}

	updateProgress("Cleaning up old worktrees...")

	// Clean up any existing worktree with same issue number (in case of previous failure)
	cleanupOldWorktrees := func() {
		entries, err := os.ReadDir("/var/tmp/vibe-kanban/worktrees")
		if err != nil {
			log.Printf("Failed to read worktrees dir: %v", err)
			return
		}
		prefix := fmt.Sprintf("%d-issue-agent", issueNumber)
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), prefix) && entry.IsDir() {
				oldPath := filepath.Join("/var/tmp/vibe-kanban/worktrees", entry.Name())
				log.Printf("Cleaning up old worktree: %s", oldPath)
				cmd := exec.Command("git", "worktree", "remove", "-f", oldPath)
				cmd.Dir = repoPath
				if out, err := cmd.CombinedOutput(); err != nil {
					log.Printf("Failed to remove old worktree %s: %v, output: %s", oldPath, err, string(out))
				}
			}
		}
	}
	cleanupOldWorktrees()

	// Ensure worktree base directory exists
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		log.Printf("Failed to create worktree base directory: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to create worktree directory: %v", err),
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	gitCmd := func(args ...string) error {
		log.Printf("git: %v", args)
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("git error: %v, output: %s", err, string(out))
		} else {
			log.Printf("git ok: %s", string(out))
		}
		return err
	}

	updateProgress("Fetching latest changes from origin...")
	// Fetch latest changes
	if err := gitCmd("fetch", "origin"); err != nil {
		log.Printf("fetch failed (continuing anyway): %v", err)
	}

	updateProgress("Determining default branch...")
	// Get default branch name
	defaultBranch := "main"
	if output, err := executeGitCommand("symbolic-ref", "refs/remotes/origin/HEAD"); err == nil {
		parts := strings.Split(strings.TrimSpace(output), "/")
		if len(parts) > 0 {
			defaultBranch = parts[len(parts)-1]
		}
	}

	updateProgress(fmt.Sprintf("Creating worktree for branch %s...", branchName))
	// Create git worktree
	log.Printf("Creating git worktree at %s from %s", worktreePath, defaultBranch)
	if err := gitCmd("worktree", "add", worktreePath, "-b", branchName, defaultBranch); err != nil {
		log.Printf("worktree creation failed: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to create worktree: %v", err),
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}
	defer func() {
		log.Printf("Removing git worktree at %s", worktreePath)
		// Use -f flag to force removal even if there are changes
		cmd := exec.Command("git", "worktree", "remove", "-f", worktreePath)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("worktree removal warning: %v, output: %s", err, string(out))
		}
	}()

	updateProgress("Checking GitHub authentication...")
	// Check if GitHub CLI is authenticated before proceeding
	// Filter out GH_TOKEN to check OAuth token
	authEnv := []string{}
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GH_TOKEN=") {
			authEnv = append(authEnv, e)
		}
	}
	authCheckCmd := exec.Command("gh", "auth", "status")
	authCheckCmd.Env = authEnv
	if err := authCheckCmd.Run(); err != nil {
		log.Printf("GitHub authentication check failed: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     "No valid GitHub CLI OAuth token detected. Please authenticate via Settings.",
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	// Create prompt for Copilot CLI
	prompt := fmt.Sprintf(`Issue #%d: %s

%s

Please implement this feature or fix.`, issueNumber, issueTitle, issueBody)

	updateProgress("Invoking GitHub Copilot CLI to analyze issue and generate implementation...")
	log.Printf("Invoking copilot CLI for issue #%d", issueNumber)

	// Use new copilot CLI (not gh extension)
	// IMPORTANT: Filter out GH_TOKEN from environment to use OAuth token
	env := []string{}
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GH_TOKEN=") {
			env = append(env, e)
		}
	}

	// Use copilot binary directly with non-interactive mode
	homeDir, _ := os.UserHomeDir()
	copilotPath := filepath.Join(homeDir, "bin", "copilot")

	// If not in home bin, try /usr/local/bin
	if _, err := os.Stat(copilotPath); os.IsNotExist(err) {
		copilotPath = "/usr/local/bin/copilot"
	}

	log.Printf("Using copilot at: %s", copilotPath)
	log.Printf("Copilot prompt: %s", prompt)

	// Add timeout context (180 minutes / 3 hours)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Minute)
	defer cancel()

	ghCmd := exec.CommandContext(ctx, copilotPath, "--allow-all-tools")
	ghCmd.Dir = worktreePath
	ghCmd.Env = env
	ghCmd.Stdin = strings.NewReader(prompt)

	// Capture output in real-time
	var ghOut bytes.Buffer
	var ghErr bytes.Buffer

	// Create pipes to read stdout/stderr in real-time
	stdoutPipe, err := ghCmd.StdoutPipe()
	if err != nil {
		log.Printf("Failed to create stdout pipe: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to setup Copilot command: %v", err),
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	stderrPipe, err := ghCmd.StderrPipe()
	if err != nil {
		log.Printf("Failed to create stderr pipe: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to setup Copilot command: %v", err),
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	// Start the command
	if err := ghCmd.Start(); err != nil {
		log.Printf("Failed to start Copilot command: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to start Copilot command: %v", err),
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	// Read stdout in real-time and update progress
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				ghOut.Write(buf[:n])
				log.Printf("Copilot stdout: %s", chunk)

				// Extract meaningful progress messages
				lines := strings.Split(chunk, "\n")
				for _, line := range lines {
					if progressMsg, ok := extractMeaningfulProgress(line); ok {
						updateProgress(fmt.Sprintf("🤖 %s", progressMsg))
					}
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// Read stderr in real-time (also check for meaningful progress messages)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				ghErr.Write(buf[:n])
				log.Printf("Copilot stderr: %s", chunk)

				// Some CLIs output progress to stderr
				lines := strings.Split(chunk, "\n")
				for _, line := range lines {
					if progressMsg, ok := extractMeaningfulProgress(line); ok {
						updateProgress(fmt.Sprintf("🤖 %s", progressMsg))
					}
				}
			}
			if err != nil {
				break
			}
		}
	}()

	err = ghCmd.Wait()

	ghOutput := ghOut.String()
	ghError := ghErr.String()

	log.Printf("gh copilot final output: %s", ghOutput)
	if ghError != "" {
		log.Printf("gh copilot final stderr: %s", ghError)
	}

	if err != nil {
		log.Printf("gh copilot error: %v", err)

		// Build detailed error message
		errorMsg := "Copilot command failed"
		errorDetails := []string{}

		if err != nil {
			errorDetails = append(errorDetails, fmt.Sprintf("Exit error: %v", err))
		}

		if ghError != "" {
			errorDetails = append(errorDetails, fmt.Sprintf("Stderr: %s", ghError))

			// Check for specific error patterns
			if strings.Contains(ghError, "code: 400") || strings.Contains(ghError, "internal server error") {
				errorMsg = "GitHub Copilot CLI returned error 400"
				errorDetails = append(errorDetails, "\nPossible causes:",
					"• CLI version compatibility issue",
					"• Copilot Pro+ features not fully supported in CLI",
					"• Rate limiting or temporary API issues",
					"\nThe Agent feature may not be available currently.")
			} else if strings.Contains(ghError, "OAuth") || strings.Contains(ghError, "not authenticated") {
				errorMsg = "Authentication error"
				errorDetails = append(errorDetails, "\nPlease authenticate via Settings.")
			}
		}

		if ghOutput != "" && len(ghOutput) < 500 {
			errorDetails = append(errorDetails, fmt.Sprintf("Output: %s", ghOutput))
		}

		fullErrorMsg := fmt.Sprintf("%s\n\n%s", errorMsg, strings.Join(errorDetails, "\n"))

		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fullErrorMsg,
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	updateProgress("Committing changes to branch...")
	// Commit changes
	log.Printf("Committing changes in worktree")
	wtGitCmd := func(args ...string) error {
		log.Printf("git (worktree): %v", args)
		cmd := exec.Command("git", args...)
		cmd.Dir = worktreePath
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("git error: %v, output: %s", err, string(out))
		} else {
			log.Printf("git ok: %s", string(out))
		}
		return err
	}

	if err := wtGitCmd("add", "."); err != nil {
		log.Printf("git add failed: %v", err)
	}

	// Check if there are any changes to commit
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = worktreePath
	statusOut, err := statusCmd.CombinedOutput()
	if err != nil {
		log.Printf("git status check failed: %v", err)
	}
	
	if len(strings.TrimSpace(string(statusOut))) == 0 {
		log.Printf("No changes to commit in worktree")
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "completed",
			Message:     "Agent completed but made no file changes. The issue may not require code modifications, or the changes were already present.",
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		
		// Clean up worktree
		log.Printf("Removing git worktree at %s", worktreePath)
		cleanupCmd := exec.Command("git", "worktree", "remove", "-f", worktreePath)
		cleanupCmd.Dir = repoPath
		if out, err := cleanupCmd.CombinedOutput(); err != nil {
			log.Printf("worktree removal warning: %v, output: %s", err, string(out))
		}
		return
	}

	commitMsg := fmt.Sprintf("Issue #%d: %s\n\nAuto-generated implementation by AirGit agent", issueNumber, issueTitle)
	if err := wtGitCmd("commit", "-m", commitMsg); err != nil {
		log.Printf("git commit failed: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to commit changes: %v", err),
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	updateProgress("Pushing branch to origin...")
	// Push branch
	if err := wtGitCmd("push", "-u", "origin", branchName); err != nil {
		log.Printf("git push failed: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to push branch: %v", err),
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	updateProgress("Creating pull request...")
	// Create PR using gh CLI
	// Note: PR creation doesn't need to be in the worktree directory
	// Use main repo path instead to avoid permission issues
	log.Printf("Creating PR for issue #%d", issueNumber)
	prTitle := fmt.Sprintf("Issue #%d: %s", issueNumber, issueTitle)
	prBody := fmt.Sprintf("Fixes #%d\n\nAuto-generated implementation by AirGit agent.", issueNumber)

	prCmd := exec.Command("gh", "pr", "create", "--title", prTitle, "--body", prBody, "--base", defaultBranch, "--head", branchName)
	prCmd.Dir = repoPath // Use main repo path, not worktree
	prCmd.Env = os.Environ()

	var prOut bytes.Buffer
	var prErr bytes.Buffer
	prCmd.Stdout = &prOut
	prCmd.Stderr = &prErr

	if err := prCmd.Run(); err != nil {
		log.Printf("PR creation failed: %v, stderr: %s", err, prErr.String())
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to create PR: %v - %s", err, prErr.String()),
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	prURL := strings.TrimSpace(prOut.String())
	log.Printf("PR created: %s", prURL)
	
	// Extract PR number from URL
	prNumber := 0
	if parts := strings.Split(prURL, "/pull/"); len(parts) == 2 {
		fmt.Sscanf(parts[1], "%d", &prNumber)
	}
	
	agentStatusMutex.Lock()
	agentStatus[issueNumber] = AgentStatus{
		IssueNumber: issueNumber,
		Status:      "completed",
		Message:     fmt.Sprintf("PR created: %s", prURL),
		PRNumber:    prNumber,
		PRURL:       prURL,
		StartTime:   startTime,
		EndTime:     time.Now(),
	}
	agentStatusMutex.Unlock()
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

	// Initialize status immediately so polling can start
	agentStatusMutex.Lock()
	agentStatus[payload.IssueNumber] = AgentStatus{
		IssueNumber: payload.IssueNumber,
		Status:      "pending",
		Message:     "Agent process queued",
		StartTime:   time.Now(),
	}
	agentStatusMutex.Unlock()

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

func handleListGitHubPRs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	remoteURL, err := exec.Command("git", "-C", config.RepoPath, "config", "--get", "remote.origin.url").Output()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Could not detect repository"})
		return
	}

	owner, repo := parseGitHubURL(strings.TrimSpace(string(remoteURL)))
	if owner == "" || repo == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Not a GitHub repository"})
		return
	}

	cmd := exec.Command("gh", "pr", "list", "--json", "number,title,state,author,createdAt,updatedAt,url,headRefName,body", "--repo", fmt.Sprintf("%s/%s", owner, repo))
	cmd.Dir = config.RepoPath
	output, err := cmd.Output()
	if err != nil {
		log.Printf("gh pr list error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Failed to list PRs", "owner": owner, "repo": repo})
		return
	}

	var prs []map[string]interface{}
	if err := json.Unmarshal(output, &prs); err != nil {
		log.Printf("parse prs error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to parse PRs"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"prs":   prs,
		"owner": owner,
		"repo":  repo,
	})
}

func handleGetPRReviews(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	prNumberStr := r.URL.Query().Get("pr_number")
	if prNumberStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing pr_number"})
		return
	}

	remoteURL, err := exec.Command("git", "-C", config.RepoPath, "config", "--get", "remote.origin.url").Output()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Could not detect repository"})
		return
	}

	owner, repo := parseGitHubURL(strings.TrimSpace(string(remoteURL)))
	if owner == "" || repo == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Not a GitHub repository"})
		return
	}

	// Get review comments with file paths using GitHub API
	cmd := exec.Command("gh", "api", fmt.Sprintf("/repos/%s/%s/pulls/%s/comments", owner, repo, prNumberStr))
	cmd.Dir = config.RepoPath
	output, err := cmd.Output()
	if err != nil {
		log.Printf("gh api error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get PR review comments"})
		return
	}

	var comments []map[string]interface{}
	if err := json.Unmarshal(output, &comments); err != nil {
		log.Printf("parse comments error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to parse PR comments"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"comments": comments,
	})
}

func handleAgentApplyReview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	var payload struct {
		IssueNumber int                      `json:"issue_number"`
		PRNumber    int                      `json:"pr_number"`
		Comments    []map[string]interface{} `json:"comments"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	log.Printf("Agent apply review: Issue #%d, PR #%d", payload.IssueNumber, payload.PRNumber)

	agentStatusMutex.Lock()
	agentStatus[payload.IssueNumber] = AgentStatus{
		IssueNumber: payload.IssueNumber,
		Status:      "pending",
		Message:     "Processing review comments",
		PRNumber:    payload.PRNumber,
		StartTime:   time.Now(),
	}
	agentStatusMutex.Unlock()

	go processReviewComments(payload.IssueNumber, payload.PRNumber, payload.Comments)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Review processing started"})
}

func processReviewComments(issueNumber, prNumber int, comments []map[string]interface{}) {
	startTime := time.Now()
	
	updateProgress := func(message string) {
		agentStatusMutex.Lock()
		if status, ok := agentStatus[issueNumber]; ok {
			status.Message = message
			agentStatus[issueNumber] = status
		}
		agentStatusMutex.Unlock()
		log.Printf("Review processing #%d: %s", issueNumber, message)
	}

	updateProgress("Getting PR information...")

	remoteURL, err := exec.Command("git", "-C", config.RepoPath, "config", "--get", "remote.origin.url").Output()
	if err != nil {
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to get repository info: %v", err),
			PRNumber:    prNumber,
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	owner, repo := parseGitHubURL(strings.TrimSpace(string(remoteURL)))
	
	var cmd *exec.Cmd
	cmd = exec.Command("gh", "pr", "view", strconv.Itoa(prNumber), "--json", "headRefName,baseRefName")
	cmd.Dir = config.RepoPath
	output, err := cmd.Output()
	if err != nil {
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to get PR info: %v", err),
			PRNumber:    prNumber,
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	var prInfo map[string]interface{}
	if err := json.Unmarshal(output, &prInfo); err != nil {
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     "Failed to parse PR info",
			PRNumber:    prNumber,
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	branchName := prInfo["headRefName"].(string)
	
	updateProgress(fmt.Sprintf("Setting up worktree for branch %s...", branchName))

	timestamp := time.Now().UnixNano() / 1000000
	worktreePath := filepath.Join("/var/tmp/vibe-kanban/worktrees", fmt.Sprintf("%04x-review-%d", timestamp&0xFFFF, timestamp))

	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to create worktree directory: %v", err),
			PRNumber:    prNumber,
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	repoPath := config.RepoPath
	gitdir, _ := exec.Command("git", "-C", repoPath, "rev-parse", "--git-dir").Output()
	if len(gitdir) > 0 {
		gitdir := strings.TrimSpace(string(gitdir))
		if strings.HasPrefix(gitdir, "gitdir: ") {
			gitdir = strings.TrimPrefix(gitdir, "gitdir: ")
			gitdir = strings.TrimSpace(gitdir)
			gitdir = filepath.Clean(filepath.Join(repoPath, gitdir))
			repoPath = filepath.Dir(filepath.Dir(gitdir))
			log.Printf("Detected worktree, using main repo: %s", repoPath)
		}
	}

	updateProgress("Fetching latest changes...")
	exec.Command("git", "-C", repoPath, "fetch", "origin").Run()

	updateProgress("Checking for existing worktrees...")
	// Check if branch is already checked out in another worktree
	listCmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	listOutput, _ := listCmd.Output()
	worktrees := string(listOutput)
	
	// Parse worktree list to find if branch is in use
	lines := strings.Split(worktrees, "\n")
	var conflictingWorktree string
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "branch ") {
			currentBranch := strings.TrimPrefix(lines[i], "branch ")
			currentBranch = strings.TrimPrefix(currentBranch, "refs/heads/")
			if currentBranch == branchName {
				// Find the worktree path (should be a few lines before)
				for j := i - 1; j >= 0 && j > i-5; j-- {
					if strings.HasPrefix(lines[j], "worktree ") {
						conflictingWorktree = strings.TrimPrefix(lines[j], "worktree ")
						break
					}
				}
				break
			}
		}
	}
	
	// Remove conflicting worktree if found
	if conflictingWorktree != "" {
		log.Printf("Found existing worktree for branch %s at %s, removing...", branchName, conflictingWorktree)
		updateProgress(fmt.Sprintf("Removing existing worktree at %s...", conflictingWorktree))
		exec.Command("git", "-C", repoPath, "worktree", "remove", "-f", conflictingWorktree).Run()
	}

	updateProgress("Creating worktree...")
	if err := exec.Command("git", "-C", repoPath, "worktree", "add", worktreePath, branchName).Run(); err != nil {
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to create worktree: %v", err),
			PRNumber:    prNumber,
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	defer func() {
		log.Printf("Removing worktree at %s", worktreePath)
		exec.Command("git", "-C", repoPath, "worktree", "remove", "--force", worktreePath).Run()
	}()

	// Check if there are any differences from the remote branch
	updateProgress("Checking differences from remote...")
	diffCmd := exec.Command("git", "-C", worktreePath, "diff", "--name-only", fmt.Sprintf("origin/%s", branchName))
	diffOutput, _ := diffCmd.Output()
	changedFiles := strings.Split(strings.TrimSpace(string(diffOutput)), "\n")
	hasLocalChanges := len(changedFiles) > 0 && changedFiles[0] != ""
	
	log.Printf("Local changes detected: %v, files: %v", hasLocalChanges, changedFiles)

	// Process delete requests first
	updateProgress("Processing file deletion requests...")
	var filesToDelete []string
	var deletedFiles []string
	var alreadyDeletedFiles []string
	
	for _, comment := range comments {
		body, bodyOk := comment["body"].(string)
		path, pathOk := comment["path"].(string)
		
		if bodyOk && pathOk && path != "" {
			// Check if comment requests file deletion
			bodyLower := strings.ToLower(body)
			if strings.Contains(bodyLower, "削除") || 
			   (strings.Contains(bodyLower, "delete") && strings.Contains(bodyLower, "file")) ||
			   (strings.Contains(bodyLower, "remove") && strings.Contains(bodyLower, "file")) {
				filesToDelete = append(filesToDelete, path)
			}
		}
	}
	
	if len(filesToDelete) > 0 {
		for _, filePath := range filesToDelete {
			fullPath := filepath.Join(worktreePath, filePath)
			// Check if file exists first
			if _, err := os.Stat(fullPath); err == nil {
				log.Printf("Deleting file: %s", fullPath)
				if err := os.Remove(fullPath); err != nil {
					log.Printf("Failed to delete file %s: %v", filePath, err)
				} else {
					log.Printf("Successfully deleted file: %s", filePath)
					deletedFiles = append(deletedFiles, filePath)
				}
			} else {
				log.Printf("File doesn't exist in current branch: %s", filePath)
				alreadyDeletedFiles = append(alreadyDeletedFiles, filePath)
			}
		}
	}
	
	// If all deletion requests are for already-deleted files and there are no other changes, skip
	if len(filesToDelete) > 0 && len(deletedFiles) == 0 && !hasLocalChanges {
		log.Printf("All requested deletions already complete, no other changes")
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "completed",
			Message:     fmt.Sprintf("Requested files already deleted: %s", strings.Join(alreadyDeletedFiles, ", ")),
			PRNumber:    prNumber,
			PRURL:       fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, prNumber),
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	updateProgress("Analyzing review comments with Copilot...")

	// Build review text for Copilot
	var reviewTextBuilder strings.Builder
	for _, comment := range comments {
		if body, ok := comment["body"].(string); ok {
			if path, ok := comment["path"].(string); ok && path != "" {
				reviewTextBuilder.WriteString(fmt.Sprintf("File: %s\n", path))
			}
			reviewTextBuilder.WriteString(body)
			reviewTextBuilder.WriteString("\n\n")
		}
	}
	reviewText := reviewTextBuilder.String()

	// Use copilot binary with same logic as processAgentIssue
	homeDir, _ := os.UserHomeDir()
	copilotPath := filepath.Join(homeDir, "bin", "copilot")
	
	if _, err := os.Stat(copilotPath); os.IsNotExist(err) {
		copilotPath = "/usr/local/bin/copilot"
	}
	
	// Check if copilot exists, if not try gh copilot
	if _, err := os.Stat(copilotPath); os.IsNotExist(err) {
		copilotPath = "gh"
	}
	
	log.Printf("Using copilot at: %s", copilotPath)
	
	prompt := fmt.Sprintf(`Review comments for PR #%d:

%s

Please analyze these review comments and apply the requested changes to the codebase.
Make the necessary code modifications to address all the feedback.`, prNumber, reviewText)

	// Filter out GH_TOKEN from environment
	env := []string{}
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GH_TOKEN=") {
			env = append(env, e)
		}
	}

	if copilotPath == "gh" {
		cmd = exec.Command("gh", "copilot", "--allow-all-tools")
	} else {
		cmd = exec.Command(copilotPath, "--allow-all-tools")
	}
	cmd.Dir = worktreePath
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Env = env

	// Create pipes to read stdout/stderr in real-time
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Failed to create stdout pipe: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to setup Copilot command: %v", err),
			PRNumber:    prNumber,
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("Failed to create stderr pipe: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to setup Copilot command: %v", err),
			PRNumber:    prNumber,
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	var copilotOut bytes.Buffer
	var copilotErr bytes.Buffer

	// Start the command
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start Copilot command: %v", err)
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to start Copilot command: %v", err),
			PRNumber:    prNumber,
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	// Read stdout in real-time and update progress
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				copilotOut.Write(buf[:n])
				log.Printf("Copilot stdout: %s", chunk)

				// Extract meaningful progress messages
				lines := strings.Split(chunk, "\n")
				for _, line := range lines {
					if progressMsg, ok := extractMeaningfulProgress(line); ok {
						updateProgress(fmt.Sprintf("🤖 %s", progressMsg))
					}
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// Read stderr in real-time
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				copilotErr.Write(buf[:n])
				log.Printf("Copilot stderr: %s", chunk)

				// Some CLIs output progress to stderr
				lines := strings.Split(chunk, "\n")
				for _, line := range lines {
					if progressMsg, ok := extractMeaningfulProgress(line); ok {
						updateProgress(fmt.Sprintf("🤖 %s", progressMsg))
					}
				}
			}
			if err != nil {
				break
			}
		}
	}()

	err = cmd.Wait()

	if err != nil {
		log.Printf("Copilot command failed: %v, stderr: %s", err, copilotErr.String())
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to process review: %v", err),
			PRNumber:    prNumber,
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	updateProgress("Committing changes...")

	exec.Command("git", "-C", worktreePath, "add", "-A").Run()
	commitMsg := fmt.Sprintf("Address review comments for PR #%d\n\nAuto-generated by AirGit agent", prNumber)
	if err := exec.Command("git", "-C", worktreePath, "commit", "-m", commitMsg).Run(); err != nil {
		log.Printf("No changes to commit after processing review comments")
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "completed",
			Message:     "No changes needed - review requests already addressed",
			PRNumber:    prNumber,
			PRURL:       fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, prNumber),
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	updateProgress("Pushing changes...")
	if err := exec.Command("git", "-C", worktreePath, "push", "origin", branchName).Run(); err != nil {
		agentStatusMutex.Lock()
		agentStatus[issueNumber] = AgentStatus{
			IssueNumber: issueNumber,
			Status:      "failed",
			Message:     fmt.Sprintf("Failed to push changes: %v", err),
			PRNumber:    prNumber,
			StartTime:   startTime,
			EndTime:     time.Now(),
		}
		agentStatusMutex.Unlock()
		return
	}

	agentStatusMutex.Lock()
	agentStatus[issueNumber] = AgentStatus{
		IssueNumber: issueNumber,
		Status:      "completed",
		Message:     "Review comments addressed and pushed",
		PRNumber:    prNumber,
		PRURL:       fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, prNumber),
		StartTime:   startTime,
		EndTime:     time.Now(),
	}
	agentStatusMutex.Unlock()
}

