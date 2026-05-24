package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

//go:embed index.html
var indexHTML embed.FS

type Task struct {
	File    string   `json:"file"`
	Status  string   `yaml:"status" json:"status"`
	Epic    string   `yaml:"epic" json:"epic"`
	Tags    []string `yaml:"tags" json:"tags"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
}

type BoardState struct {
	Tasks []Task `json:"tasks"`
	mu    sync.RWMutex
}

var board = &BoardState{}
var kanbanDir = ".kanban"
var boardFile = filepath.Join(kanbanDir, "board.json")

// Set via -ldflags at build time (see .goreleaser.yaml).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// SSE Clients
var clients = make(map[chan string]bool)
var clientsMu sync.Mutex

func main() {
	portFlag := flag.Int("port", 0, "port to listen on (default: 8080 or next available)")
	dirFlag := flag.String("dir", "docs", "directory containing markdown tasks")
	versionFlag := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("mk %s (commit %s, built %s)\n", version, commit, date)
		return
	}

	docsDir := *dirFlag

	// Ensure directories exist
	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		os.MkdirAll(docsDir, 0755)
	}
	if _, err := os.Stat(kanbanDir); os.IsNotExist(err) {
		os.Mkdir(kanbanDir, 0755)
	}

	// Initial scan
	if err := scanDocs(docsDir); err != nil {
		log.Printf("Initial scan error: %v", err)
	}

	// Setup watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write ||
					event.Op&fsnotify.Create == fsnotify.Create ||
					event.Op&fsnotify.Remove == fsnotify.Remove ||
					event.Op&fsnotify.Rename == fsnotify.Rename {
					if strings.HasSuffix(event.Name, ".md") {
						log.Printf("File changed: %s", event.Name)
						if err := scanDocs(docsDir); err != nil {
							log.Printf("Scan error: %v", err)
						}
						broadcast("update")
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(docsDir)
	if err != nil {
		log.Fatal(err)
	}

	// Find available port
	startPort := *portFlag
	if startPort == 0 {
		startPort = 8080
	}

	listener, err := findListener(startPort)
	if err != nil {
		log.Fatalf("Could not find an available port: %v", err)
	}

	actualPort := listener.Addr().(*net.TCPAddr).Port

	// HTTP Server
	http.HandleFunc("/api/board", func(w http.ResponseWriter, r *http.Request) {
		getBoard(w, r)
	})
	http.HandleFunc("/api/update", func(w http.ResponseWriter, r *http.Request) {
		updateTask(w, r, docsDir)
	})
	http.HandleFunc("/api/events", sseHandler)
	http.HandleFunc("/", serveIndex)

	fmt.Printf("MK // Kanban starting on http://localhost:%d\n", actualPort)
	fmt.Printf("Watching directory: %s\n", docsDir)
	log.Fatal(http.Serve(listener, nil))
}

func findListener(startPort int) (net.Listener, error) {
	for port := startPort; port < startPort+100; port++ {
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			return l, nil
		}
	}
	return nil, fmt.Errorf("no available ports in range %d-%d", startPort, startPort+99)
}

func updateTask(w http.ResponseWriter, r *http.Request, docsDir string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		File    string   `json:"file"`
		Content *string  `json:"content"`
		Status  string   `json:"status"`
		Tags    []string `json:"tags"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	path, err := taskPath(docsDir, req.File)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	existing, err := ioutil.ReadFile(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	parts := strings.SplitN(string(existing), "---", 3)
	var newContent string
	if len(parts) >= 3 {
		// Parse frontmatter to update status if provided
		var fm map[string]interface{}
		if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
			http.Error(w, "Invalid frontmatter", http.StatusInternalServerError)
			return
		}

		if req.Status != "" {
			fm["status"] = req.Status
		}
		if req.Tags != nil {
			fm["tags"] = req.Tags
		}

		updatedFM, _ := yaml.Marshal(fm)
		body := parts[2]
		// [P2] Distinguish between omitted and empty body
		if req.Content != nil {
			body = "\n" + *req.Content
		}
		newContent = "---\n" + string(updatedFM) + "---" + body
	} else {
		// No frontmatter, just update content or add frontmatter if status provided
		if req.Status != "" {
			content := ""
			if req.Content != nil {
				content = *req.Content
			}
			newContent = "---\nstatus: " + req.Status + "\n---\n" + content
		} else if req.Content != nil {
			newContent = *req.Content
		}
	}

	if err := ioutil.WriteFile(path, []byte(newContent), 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func taskPath(docsDir, file string) (string, error) {
	if file == "" {
		return "", fmt.Errorf("missing file")
	}
	if file != filepath.Base(file) {
		return "", fmt.Errorf("invalid file")
	}
	return filepath.Join(docsDir, file), nil
}

func broadcast(msg string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for client := range clients {
		client <- msg
	}
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	messageChan := make(chan string)
	clientsMu.Lock()
	clients[messageChan] = true
	clientsMu.Unlock()

	defer func() {
		clientsMu.Lock()
		delete(clients, messageChan)
		clientsMu.Unlock()
	}()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg := <-messageChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}

func scanDocs(docsDir string) error {
	files, err := ioutil.ReadDir(docsDir)
	if err != nil {
		return err
	}

	var newTasks []Task
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
			task, err := parseTask(filepath.Join(docsDir, f.Name()))
			if err != nil {
				log.Printf("Error parsing %s: %v", f.Name(), err)
				continue
			}
			newTasks = append(newTasks, task)
		}
	}

	board.mu.Lock()
	board.Tasks = newTasks
	board.mu.Unlock()

	// Save to board.json
	data, err := json.MarshalIndent(newTasks, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(boardFile, data, 0644)
}

func parseTask(path string) (Task, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return Task{}, err
	}

	task := Task{
		File:  filepath.Base(path),
		Title: strings.TrimSuffix(filepath.Base(path), ".md"),
	}

	// Check for Git conflict markers
	if strings.Contains(string(content), "<<<<<<<") {
		task.Status = "CONFLICT"
		task.Content = string(content)
		return task, nil
	}

	// Basic frontmatter parsing
	parts := strings.SplitN(string(content), "---", 3)
	if len(parts) >= 3 {
		err := yaml.Unmarshal([]byte(parts[1]), &task)
		if err != nil {
			return task, err
		}
		task.Content = strings.TrimSpace(parts[2])
	} else {
		task.Content = strings.TrimSpace(string(content))
	}

	return task, nil
}

func getBoard(w http.ResponseWriter, r *http.Request) {
	board.mu.RLock()
	defer board.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(board.Tasks)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := indexHTML.ReadFile("index.html")
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}
