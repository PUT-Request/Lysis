package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"lysis/internal/ai"
	"lysis/internal/cache"
	"lysis/internal/config"
	"lysis/internal/db"
	"lysis/internal/middleware"
	"lysis/internal/scanner"
	"lysis/internal/worker"
)

type Handlers struct {
	DB       *sql.DB
	Config   *config.Config
	AIClient ai.ChatClient
	Cache    *cache.HashCache
	Pool     *worker.Pool
	Limiter  *middleware.RateLimiter
}

func (h *Handlers) CreateScan(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 201*1024*1024)

	if err := r.ParseMultipartForm(32 * 1024 * 1024); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("file too large or invalid form"))
		return
	}

	scanType := r.FormValue("type")
	if scanType != "exploit" && scanType != "malware" {
		writeJSON(w, http.StatusBadRequest, errResp("type must be 'exploit' or 'malware'"))
		return
	}

	maxUploadSize := int64(50) * 1024 * 1024
	if scanType == "malware" {
		maxUploadSize = int64(200) * 1024 * 1024
	}

	allowed, reason := h.Limiter.AllowNewScan(user.UserID)
	if !allowed {
		writeJSON(w, http.StatusTooManyRequests, errResp(reason))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.Limiter.ReleaseScan(user.UserID)
		writeJSON(w, http.StatusBadRequest, errResp("no file provided"))
		return
	}
	defer file.Close()

	if header.Size > maxUploadSize {
		h.Limiter.ReleaseScan(user.UserID)
		writeJSON(w, http.StatusBadRequest, errResp("file exceeds maximum size"))
		return
	}

	scanID := uuid.New().String()
	scanDir := filepath.Join(h.Config.Sandbox.TempDir, scanID)

	safeName := filepath.Base(header.Filename)
	if safeName == "" || safeName == "." || safeName == "/" {
		safeName = "upload"
	}
	filePath := filepath.Join(scanDir, safeName)
	if err := os.MkdirAll(scanDir, 0700); err != nil {
		h.Limiter.ReleaseScan(user.UserID)
		logError("create scan dir", err)
		writeJSON(w, http.StatusInternalServerError, errResp("failed to create scan"))
		return
	}

	out, err := os.Create(filePath)
	if err != nil {
		h.Limiter.ReleaseScan(user.UserID)
		logError("save file", err)
		writeJSON(w, http.StatusInternalServerError, errResp("failed to upload file"))
		return
	}

	copied, err := io.Copy(out, file)
	out.Close()
	if err != nil || copied == 0 {
		h.Limiter.ReleaseScan(user.UserID)
		os.Remove(filePath)
		logError("copy upload", err)
		writeJSON(w, http.StatusInternalServerError, errResp("failed to upload file"))
		return
	}

	hash := computeFileHash(filePath)

	sourceDetail := safeName
	scan := &db.Scan{
		ID:              scanID,
		UserID:          user.UserID,
		Type:            scanType,
		Source:          "upload",
		SourceDetail:    &sourceDetail,
		Status:          "queued",
		FileHash:        &hash,
		ShareVisibility: "private",
	}

	if err := db.InsertScan(h.DB, scan); err != nil {
		h.Limiter.ReleaseScan(user.UserID)
		logError("insert scan", err)
		writeJSON(w, http.StatusInternalServerError, errResp("failed to create scan"))
		return
	}

	job := worker.Job{
		ScanID: scanID,
		UserID: user.UserID,
		Run: func(ctx context.Context) error {
			defer h.Limiter.ReleaseScan(user.UserID)
			return h.runScan(ctx, scanID, scanType, filePath)
		},
	}

	if !h.Pool.Submit(job) {
		db.UpdateScanStatus(h.DB, scanID, "failed", "server busy, try again")
		h.Limiter.ReleaseScan(user.UserID)
		writeJSON(w, http.StatusServiceUnavailable, errResp("server busy, try again later"))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"scan_id": scanID})
}

func (h *Handlers) ListScans(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized"))
		return
	}

	page := 1
	limit := 10
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	result, err := db.ListScans(h.DB, db.ScanListParams{
		UserID: user.UserID,
		Page:   page,
		Limit:  limit,
		Search: search,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("failed to list scans"))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) GetScan(w http.ResponseWriter, r *http.Request) {
	scanID := extractScanID(r.URL.Path, "/api/scans/", "")

	user := getUser(r)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized"))
		return
	}

	scan, err := db.GetScanByID(h.DB, scanID)
	if err != nil || scan == nil {
		writeJSON(w, http.StatusNotFound, errResp("scan not found"))
		return
	}

	if scan.UserID != user.UserID {
		writeJSON(w, http.StatusForbidden, errResp("access denied"))
		return
	}

	writeJSON(w, http.StatusOK, scan)
}

func (h *Handlers) GetScanStatus(w http.ResponseWriter, r *http.Request) {
	scanID := extractScanID(r.URL.Path, "/api/scans/", "/status")

	user := getUser(r)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized"))
		return
	}

	scan, err := db.GetScanByID(h.DB, scanID)
	if err != nil || scan == nil {
		writeJSON(w, http.StatusNotFound, errResp("scan not found"))
		return
	}

	if scan.UserID != user.UserID {
		writeJSON(w, http.StatusForbidden, errResp("access denied"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": scan.Status,
	})
}

func (h *Handlers) DeleteScan(w http.ResponseWriter, r *http.Request) {
	scanID := extractScanID(r.URL.Path, "/api/scans/", "")

	user := getUser(r)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized"))
		return
	}

	scan, err := db.GetScanByID(h.DB, scanID)
	if err != nil || scan == nil {
		writeJSON(w, http.StatusNotFound, errResp("scan not found"))
		return
	}

	if scan.UserID != user.UserID {
		writeJSON(w, http.StatusForbidden, errResp("access denied"))
		return
	}

	_ = db.DeleteScan(h.DB, scanID)
	os.RemoveAll(filepath.Join(h.Config.Sandbox.TempDir, scanID))

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handlers) runScan(ctx context.Context, scanID, scanType, filePath string) error {
	db.UpdateScanRunning(h.DB, scanID)

	sandbox, err := scanner.NewSandbox(scanID, scanner.SandboxConfig{
		TempDir:            h.Config.Sandbox.TempDir,
		Tools:              h.Config.Sandbox.Tools,
		BwrapPath:          h.Config.Sandbox.BwrapPath,
		CommandTimeoutSecs: h.Config.Sandbox.CommandTimeoutSecs,
		MaxMemoryMB:        h.Config.Sandbox.MaxMemoryMB,
		CleanupOnComplete:  h.Config.Sandbox.CleanupOnComplete,
	})
	if err != nil {
		logError("sandbox init", err)
		db.UpdateScanStatus(h.DB, scanID, "failed", "sandbox initialization failed")
		return err
	}
	defer sandbox.Destroy()

	if err := sandbox.Create(ctx); err != nil {
		logError("sandbox create", err)
		db.UpdateScanStatus(h.DB, scanID, "failed", "sandbox setup failed")
		return err
	}

	var files []string
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".zip" {
		maxBytes := int64(h.Config.Limits.MaxZipUncompressedMB) * 1024 * 1024
		extracted, err := scanner.ExtractZip(filePath, sandbox.HostPath(), maxBytes)
		if err != nil {
			logError("zip extraction", err)
			db.UpdateScanStatus(h.DB, scanID, "failed", "file extraction failed")
			return err
		}
		files = extracted
	} else {
		files = []string{filePath}
	}

	var resultJSON []byte
	var prescanJSON string
	var verdict string

	if scanType == "exploit" {
		es := scanner.NewExploitScanner(*h.Config, h.AIClient)
		result, err := es.Run(ctx, sandbox, files)
		if err != nil {
			logError("exploit scan", err)
			db.UpdateScanStatus(h.DB, scanID, "failed", "analysis failed")
			return err
		}
		resultJSON = result

		var parsed ai.ExploitResult
		json.Unmarshal(result, &parsed)
		if len(parsed.Findings) > 0 && parsed.Findings[0].Severity != "" {
			verdict = parsed.Findings[0].Severity
		}
	} else {
		ms := scanner.NewMalwareScanner(*h.Config, h.AIClient, h.Cache)
		result, prescan, err := ms.Run(ctx, sandbox, files)
		if err != nil {
			logError("malware scan", err)
			db.UpdateScanStatus(h.DB, scanID, "failed", "analysis failed")
			return err
		}
		resultJSON = result
		prescanJSON = prescan

		var parsed ai.MalwareResult
		json.Unmarshal(result, &parsed)
		verdict = parsed.Verdict
	}

	db.UpdateScanResult(h.DB, scanID, verdict, prescanJSON, string(resultJSON))
	return nil
}

func computeFileHash(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}

func logError(context string, err error) {
	if err != nil {
		log.Printf("[scans] %s: %v", context, err)
	}
}

func (h *Handlers) scanTimeout() time.Duration {
	d, err := time.ParseDuration(h.Config.Limits.ScanTimeout)
	if err != nil || d <= 0 {
		return 30 * time.Minute
	}
	return d
}
