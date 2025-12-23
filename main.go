package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

//go:embed static/*
var staticFiles embed.FS

type Config struct {
	SSHHost       string
	SSHPort       string
	SSHUser       string
	SSHKeyPath    string
	RepoPath      string
	ListenAddr    string
	ListenPort    string
}

type Response struct {
	Branch string      `json:"branch,omitempty"`
	Server string      `json:"server,omitempty"`
	Error  string      `json:"error,omitempty"`
	Log    []string    `json:"log,omitempty"`
	Commit string      `json:"commit,omitempty"`
}

var config Config

func init() {
	config = Config{
		SSHHost:    getEnv("AIRGIT_SSH_HOST", "localhost"),
		SSHPort:    getEnv("AIRGIT_SSH_PORT", "22"),
		SSHUser:    getEnv("AIRGIT_SSH_USER", "git"),
		SSHKeyPath: getEnv("AIRGIT_SSH_KEY", filepath.Join(os.Getenv("HOME"), ".ssh/id_rsa")),
		RepoPath:   getEnv("AIRGIT_REPO_PATH", "/var/git/repo"),
		ListenAddr: getEnv("AIRGIT_LISTEN_ADDR", "0.0.0.0"),
		ListenPort: getEnv("AIRGIT_LISTEN_PORT", "8080"),
	}

	log.Printf("Config: Host=%s Port=%s User=%s RepoPath=%s", 
		config.SSHHost, config.SSHPort, config.SSHUser, config.RepoPath)
}

func getEnv(key, defaultVal string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultVal
}

func main() {
	http.HandleFunc("/", serveStatic)
	http.HandleFunc("/manifest.json", serveManifest)
	http.HandleFunc("/service-worker.js", serveServiceWorker)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/push", handlePush)

	addr := net.JoinHostPort(config.ListenAddr, config.ListenPort)
	log.Printf("Starting AirGit on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data, _ := staticFiles.ReadFile("static/index.html")
		w.Write(data)
		return
	}
	http.NotFound(w, r)
}

func serveManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	data, _ := staticFiles.ReadFile("static/manifest.json")
	w.Write(data)
}

func serveServiceWorker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	data, _ := staticFiles.ReadFile("static/service-worker.js")
	w.Write(data)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	branch, err := executeGitCommand("branch", "--show-current")
	if err != nil {
		json.NewEncoder(w).Encode(Response{
			Error: fmt.Sprintf("Failed to get branch: %v", err),
		})
		return
	}

	branch = strings.TrimSpace(branch)
	serverInfo := fmt.Sprintf("%s@%s", config.SSHUser, config.SSHHost)

	json.NewEncoder(w).Encode(Response{
		Branch: branch,
		Server: serverInfo,
	})
}

func handlePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

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
	if err != nil {
		resp := Response{
			Error: fmt.Sprintf("Failed to get branch: %v", err),
			Log:   logs,
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
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

func executeGitCommand(args ...string) (string, error) {
	sshConfig, err := createSSHConfig()
	if err != nil {
		return "", fmt.Errorf("SSH config error: %v", err)
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(config.SSHHost, config.SSHPort), sshConfig)
	if err != nil {
		return "", fmt.Errorf("SSH dial error: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("SSH session error: %v", err)
	}
	defer session.Close()

	// Build git command
	gitCmd := fmt.Sprintf("cd %s && git %s", 
		shellQuote(config.RepoPath), 
		strings.Join(quoteArgs(args), " "))

	var output bytes.Buffer
	session.Stdout = &output
	session.Stderr = &output

	err = session.Run(gitCmd)

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

func createSSHConfig() (*ssh.ClientConfig, error) {
	keyBytes, err := os.ReadFile(config.SSHKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key: %v", err)
	}

	return &ssh.ClientConfig{
		User: config.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil // Disable host key verification for simplicity
		},
	}, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func quoteArgs(args []string) []string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}
	return quoted
}
