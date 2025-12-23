package main

import (
	"bytes"
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
	"strings"
)

const version = "1.0.0"

//go:embed static/*
var staticFiles embed.FS

type Config struct {
	RepoPath   string
	ListenAddr string
	ListenPort string
}

type Response struct {
	Branch   string      `json:"branch,omitempty"`
	RepoName string      `json:"repoName,omitempty"`
	Error    string      `json:"error,omitempty"`
	Log      []string    `json:"log,omitempty"`
	Commit   string      `json:"commit,omitempty"`
	Branches []string    `json:"branches,omitempty"`
	Remotes  []string    `json:"remotes,omitempty"`
	Ahead    int         `json:"ahead,omitempty"`
	Behind   int         `json:"behind,omitempty"`
}

type RemoteInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Repository struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

var config Config
var baseRepoPath string

func init() {
	config = Config{
		RepoPath:   getEnv("AIRGIT_REPO_PATH", os.Getenv("HOME")),
		ListenAddr: getEnv("AIRGIT_LISTEN_ADDR", "0.0.0.0"),
		ListenPort: getEnv("AIRGIT_LISTEN_PORT", "8080"),
	}
	baseRepoPath = config.RepoPath

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

	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.BoolVar(&showHelp, "h", false, "Show help message (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.BoolVar(&showVersion, "v", false, "Show version (shorthand)")
	flag.StringVar(&repoPath, "repo-path", "", "Absolute path to Git repository (default: $HOME)")
	flag.StringVar(&listenAddr, "listen-addr", "", "Server listen address (default: 0.0.0.0)")
	flag.StringVar(&listenPort, "listen-port", "", "Server listen port (default: 8080)")
	flag.StringVar(&listenPort, "port", "", "Server listen port (alias for --listen-port, default: 8080)")
	flag.StringVar(&listenPort, "p", "", "Server listen port (shorthand, default: 8080)")

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

	http.HandleFunc("/manifest.json", serveManifest)
	http.HandleFunc("/service-worker.js", serveServiceWorker)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/push", handlePush)
	http.HandleFunc("/api/pull", handlePull)
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
	http.HandleFunc("/api/systemd/register", handleSystemdRegister)
	http.HandleFunc("/api/systemd/status", handleSystemdStatus)
	http.HandleFunc("/", serveRoot)

	addr := net.JoinHostPort(config.ListenAddr, config.ListenPort)
	log.Printf("Starting AirGit on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
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

Examples:
  # Using environment variables
  export AIRGIT_REPO_PATH=/path/to/repo
  airgit

  # Using command-line flags
  airgit --repo-path /path/to/repo

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
	data, err := staticFiles.ReadFile("static/manifest.json")
	if err != nil {
		http.Error(w, "Failed to read manifest.json", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

func serveServiceWorker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	data, err := staticFiles.ReadFile("static/service-worker.js")
	if err != nil {
		http.Error(w, "Failed to read service-worker.js", http.StatusInternalServerError)
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
StandardError=journal

[Install]
WantedBy=default.target
`, execPath)

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
