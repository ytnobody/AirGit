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
	Branch  string      `json:"branch,omitempty"`
	Error   string      `json:"error,omitempty"`
	Log     []string    `json:"log,omitempty"`
	Commit  string      `json:"commit,omitempty"`
	Branches []string   `json:"branches,omitempty"`
}

type Repository struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

var config Config

func init() {
	config = Config{
		RepoPath:   getEnv("AIRGIT_REPO_PATH", os.Getenv("HOME")),
		ListenAddr: getEnv("AIRGIT_LISTEN_ADDR", "0.0.0.0"),
		ListenPort: getEnv("AIRGIT_LISTEN_PORT", "8080"),
	}

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
	http.HandleFunc("/api/repos", handleListRepos)
	http.HandleFunc("/api/load-repo", handleLoadRepo)
	http.HandleFunc("/api/branch/create", handleCreateBranch)
	http.HandleFunc("/api/branches", handleListBranches)
	http.HandleFunc("/api/checkout", handleCheckoutBranch)
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

	json.NewEncoder(w).Encode(Response{
		Branch: branch,
	})
}

func handlePush(w http.ResponseWriter, r *http.Request) {
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

	var logs []string

	// git add .
	output, err := executeGitCommand("add", ".")
	logs = append(logs, "$ git add .")
	if output != "" {
		logs = append(logs, output)
	}
	if err != nil {
		resp := Response{
			Error: fmt.Sprintf("git add failed: %v", err),
			Log:   logs,
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// git commit
	output, err = executeGitCommand("commit", "-m", "Updated via AirGit")
	logs = append(logs, "$ git commit -m \"Updated via AirGit\"")
	if output != "" {
		logs = append(logs, output)
	}
	if err != nil {
		// Commit may fail if nothing to commit, which is ok
		if !strings.Contains(err.Error(), "nothing to commit") {
			resp := Response{
				Error: fmt.Sprintf("git commit failed: %v", err),
				Log:   logs,
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(resp)
			return
		}
		logs = append(logs, "(nothing to commit)")
	}

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

	// git push origin [branch]
	output, err = executeGitCommand("push", "origin", branch)
	logs = append(logs, fmt.Sprintf("$ git push origin %s", branch))
	if output != "" {
		logs = append(logs, output)
	}
	if err != nil {
		resp := Response{
			Error: fmt.Sprintf("git push failed: %v", err),
			Log:   logs,
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	logs = append(logs, "✓ Push successful!")

	json.NewEncoder(w).Encode(Response{
		Branch: branch,
		Log:    logs,
	})
}

func handleListRepos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	log.Printf("ListRepos: scanning path=%s", config.RepoPath)
	repos, err := listRepositories(config.RepoPath)
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
	resolvedPath := filepath.Join(config.RepoPath, req.RelativePath)
	resolvedPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid path",
		})
		return
	}

	// Check that resolved path is within the base repo path
	basePath, _ := filepath.Abs(config.RepoPath)
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

	json.NewEncoder(w).Encode(Response{
		Branch: branch,
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
		resolvedPath := filepath.Join(config.RepoPath, req.RelativePath)
		resolvedPath, err := filepath.Abs(resolvedPath)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid path",
			})
			return
		}

		// Check that resolved path is within the base repo path
		basePath, _ := filepath.Abs(config.RepoPath)
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
			resolvedPath := filepath.Join(config.RepoPath, relativePath)
			resolvedPath, err := filepath.Abs(resolvedPath)
			if err == nil {
				// Check that resolved path is within the base repo path
				basePath, _ := filepath.Abs(config.RepoPath)
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

	json.NewEncoder(w).Encode(map[string]interface{}{
		"branch": currentBranch,
	})
}

func handleInit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	relativePath := r.URL.Query().Get("relativePath")
	branch := r.URL.Query().Get("branch")

	// If relativePath is provided, update repo path safely
	if relativePath != "" {
		// Resolve the relative path safely to prevent directory traversal
		resolvedPath := filepath.Join(config.RepoPath, relativePath)
		resolvedPath, err := filepath.Abs(resolvedPath)
		if err == nil {
			// Check that resolved path is within the base repo path
			basePath, _ := filepath.Abs(config.RepoPath)
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

	json.NewEncoder(w).Encode(map[string]interface{}{
		"branch": currentBranch,
	})
}
