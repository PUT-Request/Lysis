package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"syscall"

	"lysis/internal/auth"
	"lysis/internal/db"
)

func getUser(r *http.Request) *auth.UserClaims {
	return auth.GetUser(r)
}

func (h *Handlers) Stats(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized"))
		return
	}

	stats, err := db.GetUserStats(h.DB, user.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("failed to get stats"))
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	diskFree := h.getDiskFree()
	ok := diskFree >= h.Config.Disk.MinFreeBytes
	status := "ok"
	if !ok {
		status = "disk_full"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":          status,
		"disk_free_bytes": diskFree,
	})
}

func (h *Handlers) getDiskFree() int64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(h.Config.Sandbox.TempDir, &stat); err != nil {
		return 0
	}
	return int64(stat.Bavail) * int64(stat.Bsize)
}

func (h *Handlers) ShareScan(w http.ResponseWriter, r *http.Request) {
	scanID := extractScanID(r.URL.Path, "/api/scans/", "/share")

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

	var req struct {
		Visibility string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid request"))
		return
	}

	if req.Visibility != "private" && req.Visibility != "logged_in" && req.Visibility != "public" {
		writeJSON(w, http.StatusBadRequest, errResp("visibility must be private, logged_in, or public"))
		return
	}

	var tokenPtr *string
	if req.Visibility != "private" {
		t := generateShareToken()
		tokenPtr = &t
	}

	shareToken := ""
	if tokenPtr != nil {
		shareToken = *tokenPtr
	}

	if err := db.UpdateShareVisibility(h.DB, scanID, req.Visibility, tokenPtr); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("failed to update share settings"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"visibility":  req.Visibility,
		"share_token": shareToken,
	})
}

func (h *Handlers) SharedScan(w http.ResponseWriter, r *http.Request) {
	token := extractScanID(r.URL.Path, "/api/shared/", "")

	scan, err := db.GetScanByShareToken(h.DB, token)
	if err != nil || scan == nil {
		writeJSON(w, http.StatusNotFound, errResp("scan not found"))
		return
	}

	if scan.ShareVisibility == "logged_in" {
		if getUser(r) == nil {
			writeJSON(w, http.StatusUnauthorized, errResp("login required to view this scan"))
			return
		}
	}

	writeJSON(w, http.StatusOK, scan)
}

func extractScanID(path, prefix, suffix string) string {
	s := strings.TrimPrefix(path, prefix)
	if suffix != "" {
		s = strings.TrimSuffix(s, suffix)
	}
	parts := strings.Split(s, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func generateShareToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func errResp(msg string) map[string]string {
	return map[string]string{"error": msg}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
