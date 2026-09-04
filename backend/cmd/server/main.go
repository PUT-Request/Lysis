package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"lysis/internal/ai"
	"lysis/internal/api"
	"lysis/internal/auth"
	"lysis/internal/cache"
	"lysis/internal/config"
	"lysis/internal/db"
	"lysis/internal/middleware"
	"lysis/internal/worker"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logDir := filepath.Dir(*configPath)
	logFile, err := os.OpenFile(filepath.Join(logDir, "error.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	}

	database, err := db.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	primaryClient := ai.NewClient(cfg.LLM)
	var secondaryClient *ai.Client
	if cfg.LLMSecondary.APIKey != "" {
		secondaryClient = ai.NewClient(cfg.LLMSecondary)
	}
	aiClient := ai.NewFailoverClient(primaryClient, secondaryClient)
	hashCache := cache.NewHashCache(database)
	workerPool := worker.NewPool(cfg.Limits)
	rateLimiter := middleware.NewRateLimiter(cfg.Limits)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	workerPool.Start(ctx, cfg.Limits.MaxGlobalConcurrent)

	h := &api.Handlers{
		DB:       database,
		Config:   cfg,
		AIClient: aiClient,
		Cache:    hashCache,
		Pool:     workerPool,
		Limiter:  rateLimiter,
	}

	authHandler := auth.NewHandler(database, cfg)
	authMiddleware := auth.Middleware(cfg.Auth.JWTSecret)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", h.Health)

	mux.HandleFunc("/api/auth/signup", authHandler.Signup)
	mux.HandleFunc("/api/auth/login", authHandler.Login)
	mux.Handle("/api/auth/me", authMiddleware(http.HandlerFunc(authHandler.Me)))

	mux.Handle("/api/stats", authMiddleware(http.HandlerFunc(h.Stats)))

	mux.Handle("/api/scans/github", authMiddleware(http.HandlerFunc(h.GitHubScan)))
	mux.Handle("/api/scans", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.CreateScan(w, r)
		case http.MethodGet:
			h.ListScans(w, r)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})))

	mux.Handle("/api/scans/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/status") {
			h.GetScanStatus(w, r)
		} else if strings.HasSuffix(path, "/share") {
			h.ShareScan(w, r)
		} else {
			switch r.Method {
			case http.MethodGet:
				h.GetScan(w, r)
			case http.MethodDelete:
				h.DeleteScan(w, r)
			default:
				writeJSON(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		}
	})))

	mux.HandleFunc("/api/shared/", h.SharedScan)

	staticDir := cfg.Server.StaticDir
	if staticDir == "" {
		staticDir = "frontend/out"
	}
	absStatic, _ := filepath.Abs(staticDir)
	if info, err := os.Stat(absStatic); err == nil && info.IsDir() {
		fs := noDirFileServer{root: http.Dir(absStatic)}
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isAPIPath(r.URL.Path) {
				http.NotFound(w, r)
				return
			}
			fs.ServeHTTP(w, r)
		}))
		log.Printf("Serving static files from %s", absStatic)
	}

	handler := corsMiddleware(mux)

	addr := addrString(cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Lysis server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}
	log.Println("Server stopped")
}

func addrString(host string, port int) string {
	if host == "" {
		host = "0.0.0.0"
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func isAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/")
}

type noDirFileServer struct {
	root http.FileSystem
}

func (fs noDirFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := filepath.Clean(r.URL.Path)

	if strings.HasPrefix(filepath.Base(path), "__next") {
		http.NotFound(w, r)
		return
	}

	if path == "/" {
		path = "/index.html"
	}

	trimmed := strings.TrimSuffix(path, "/")
	htmlPath := trimmed + ".html"

	if f, err := fs.root.Open(htmlPath); err == nil {
		f.Close()
		http.ServeFile(w, r, filepath.Join(string(fs.root.(http.Dir)), htmlPath))
		return
	}

	file, err := fs.root.Open(path)
	if err != nil {
		if strings.HasPrefix(r.URL.Path, "/_next/") {
			http.NotFound(w, r)
			return
		}
		htmlPath2 := path + ".html"
		if f, err2 := fs.root.Open(htmlPath2); err2 == nil {
			f.Close()
			http.ServeFile(w, r, filepath.Join(string(fs.root.(http.Dir)), htmlPath2))
			return
		}
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if stat.IsDir() {
		indexPath := path + "/index.html"
		if f, err2 := fs.root.Open(indexPath); err2 == nil {
			f.Close()
			http.ServeFile(w, r, filepath.Join(string(fs.root.(http.Dir)), indexPath))
			return
		}
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, filepath.Join(string(fs.root.(http.Dir)), path))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "https://lysis.kernal.bid"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
