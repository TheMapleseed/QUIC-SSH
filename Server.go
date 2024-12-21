package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/lucas-clemente/quic-go/http3"
)

// Operation represents a validated command request
type Operation struct {
	Action     string            `json:"action"`
	Parameters map[string]string `json:"parameters"`
	Timestamp  time.Time         `json:"timestamp"`
}

// Response represents the server's response
type Response struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// Config holds server configuration
type Config struct {
	AllowedPaths     []string          `json:"allowed_paths"`
	AllowedActions   map[string]bool   `json:"allowed_actions"`
	MaxFileSize      int64            `json:"max_file_size"`
	AllowedFileTypes []string          `json:"allowed_file_types"`
}

var (
	config     Config
	jwtSecret  = []byte(os.Getenv("JWT_SECRET"))
)

func init() {
	// Initialize server configuration
	config = Config{
		AllowedPaths: []string{
			"/var/www/public",
			"/data/shared",
		},
		AllowedActions: map[string]bool{
			"list_files":    true,
			"read_file":     true,
			"write_file":    true,
			"create_folder": true,
		},
		MaxFileSize: 10 * 1024 * 1024, // 10MB
		AllowedFileTypes: []string{
			".txt", ".json", ".csv", ".log",
		},
	}
}

func validateToken(token string) (*jwt.Token, error) {
	return jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := validateToken(tokenString)
		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func operationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var op Operation
	if err := json.NewDecoder(r.Body).Decode(&op); err != nil {
		sendResponse(w, Response{
			Status:  "error",
			Message: "Invalid request format",
		}, http.StatusBadRequest)
		return
	}

	// Validate operation
	if !config.AllowedActions[op.Action] {
		sendResponse(w, Response{
			Status:  "error",
			Message: "Operation not allowed",
		}, http.StatusForbidden)
		return
	}

	// Process operation
	result, err := processOperation(op)
	if err != nil {
		sendResponse(w, Response{
			Status:  "error",
			Message: err.Error(),
		}, http.StatusInternalServerError)
		return
	}

	sendResponse(w, Response{
		Status: "success",
		Data:   result,
	}, http.StatusOK)
}

func processOperation(op Operation) (interface{}, error) {
	switch op.Action {
	case "list_files":
		return listFiles(op.Parameters["path"])
	case "read_file":
		return readFile(op.Parameters["path"])
	case "write_file":
		return writeFile(op.Parameters["path"], op.Parameters["content"])
	case "create_folder":
		return createFolder(op.Parameters["path"])
	default:
		return nil, fmt.Errorf("unsupported operation")
	}
}

func listFiles(path string) ([]string, error) {
	if !isPathAllowed(path) {
		return nil, fmt.Errorf("access denied to path: %s", path)
	}

	files, err := filepath.Glob(filepath.Join(path, "*"))
	if err != nil {
		return nil, err
	}

	return files, nil
}

func readFile(path string) (string, error) {
	if !isPathAllowed(path) {
		return "", fmt.Errorf("access denied to path: %s", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func writeFile(path, content string) (bool, error) {
	if !isPathAllowed(path) {
		return false, fmt.Errorf("access denied to path: %s", path)
	}

	if !isFileTypeAllowed(path) {
		return false, fmt.Errorf("file type not allowed")
	}

	err := os.WriteFile(path, []byte(content), 0644)
	return err == nil, err
}

func createFolder(path string) (bool, error) {
	if !isPathAllowed(path) {
		return false, fmt.Errorf("access denied to path: %s", path)
	}

	err := os.MkdirAll(path, 0755)
	return err == nil, err
}

func isPathAllowed(path string) bool {
	path = filepath.Clean(path)
	for _, allowedPath := range config.AllowedPaths {
		if strings.HasPrefix(path, allowedPath) {
			return true
		}
	}
	return false
}

func isFileTypeAllowed(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, allowedType := range config.AllowedFileTypes {
		if ext == allowedType {
			return true
		}
	}
	return false
}

func sendResponse(w http.ResponseWriter, resp Response, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func main() {
	// Set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("/api/operation", authMiddleware(operationHandler))

	// Configure HTTP/3 server
	server := &http3.Server{
		Addr:    ":443",
		Handler: mux,
	}

	// Start server
	log.Println("Starting secure HTTP/3 server on :443...")
	err := server.ListenAndServeTLS("server.crt", "server.key")
	if err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
