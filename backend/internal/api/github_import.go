package api

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"lysis/internal/db"
	"lysis/internal/worker"
)

type githubRequest struct {
	URL             string `json:"url"`
	Type            string `json:"type"`
	IncludeReleases bool   `json:"include_releases"`
}

type githubRelease struct {
	TagName string         `json:"tag_name"`
	Assets  []githubAsset  `json:"assets"`
}

type githubAsset struct {
	Name                string `json:"name"`
	BrowserDownloadURL  string `json:"browser_download_url"`
	Size                int64  `json:"size"`
}

func (h *Handlers) GitHubScan(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errResp("unauthorized"))
		return
	}

	var req githubRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid request body"))
		return
	}

	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, errResp("github URL required"))
		return
	}

	owner, repo, err := parseGitHubURL(req.URL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(err.Error()))
		return
	}

	if req.Type != "exploit" && req.Type != "malware" {
		writeJSON(w, http.StatusBadRequest, errResp("type must be 'exploit' or 'malware'"))
		return
	}

	allowed, reason := h.Limiter.AllowNewScan(user.UserID)
	if !allowed {
		writeJSON(w, http.StatusTooManyRequests, errResp(reason))
		return
	}

	scanID := uuid.New().String()
	scanDir := filepath.Join(h.Config.Sandbox.TempDir, scanID)
	repoURL := fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)

	if err := os.MkdirAll(scanDir, 0755); err != nil {
		h.Limiter.ReleaseScan(user.UserID)
		writeJSON(w, http.StatusInternalServerError, errResp("failed to create scan dir"))
		return
	}

	cloneDir := filepath.Join(scanDir, "repo")
	cloneCtx, cloneCancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cloneCancel()

	if err := cloneRepo(cloneCtx, repoURL, cloneDir); err != nil {
		h.Limiter.ReleaseScan(user.UserID)
		os.RemoveAll(scanDir)
		log.Printf("[github] clone failed: %v", err)
		writeJSON(w, http.StatusBadRequest, errResp("repository clone failed"))
		return
	}

	if req.IncludeReleases && req.Type == "malware" {
		releaseDir := filepath.Join(scanDir, "releases")
		os.MkdirAll(releaseDir, 0755)
		if err := fetchLatestReleaseAssets(r.Context(), owner, repo, releaseDir); err != nil {
			// non-fatal: continue without releases
		}
	}

	zipPath := filepath.Join(scanDir, "repo.zip")
	if err := createZipFromDir(cloneDir, zipPath); err != nil {
		h.Limiter.ReleaseScan(user.UserID)
		os.RemoveAll(scanDir)
		writeJSON(w, http.StatusInternalServerError, errResp("failed to archive repo"))
		return
	}

	os.RemoveAll(cloneDir)
	releaseDir := filepath.Join(scanDir, "releases")
	os.RemoveAll(releaseDir)

	sourceDetail := fmt.Sprintf("github.com/%s/%s", owner, repo)
	if req.IncludeReleases {
		sourceDetail += " (+releases)"
	}

	scan := &db.Scan{
		ID:              scanID,
		UserID:          user.UserID,
		Type:            req.Type,
		Source:          "github",
		SourceDetail:    &sourceDetail,
		Status:          "queued",
		ShareVisibility: "private",
	}

	if err := db.InsertScan(h.DB, scan); err != nil {
		h.Limiter.ReleaseScan(user.UserID)
		os.RemoveAll(scanDir)
		writeJSON(w, http.StatusInternalServerError, errResp("failed to create scan"))
		return
	}

	job := worker.Job{
		ScanID: scanID,
		UserID: user.UserID,
		Run: func(ctx context.Context) error {
			defer h.Limiter.ReleaseScan(user.UserID)
			return h.runScan(ctx, scanID, req.Type, zipPath)
		},
	}

	if !h.Pool.Submit(job) {
		db.UpdateScanStatus(h.DB, scanID, "failed", "worker queue full")
		h.Limiter.ReleaseScan(user.UserID)
		os.RemoveAll(scanDir)
		writeJSON(w, http.StatusServiceUnavailable, errResp("server busy, try again later"))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"scan_id": scanID})
}

var githubURLRe = regexp.MustCompile(`^(https?://)?(www\.)?github\.com/([a-zA-Z0-9._-]+)/([a-zA-Z0-9._-]+)(\.git)?/?$`)

func parseGitHubURL(raw string) (owner, repo string, err error) {
	raw = strings.TrimSpace(raw)
	matches := githubURLRe.FindStringSubmatch(raw)
	if len(matches) < 5 {
		return "", "", fmt.Errorf("invalid github URL: expected https://github.com/owner/repo")
	}
	owner = matches[3]
	repo = matches[4]
	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("invalid github URL: owner and repo required")
	}
	return owner, repo, nil
}

func cloneRepo(ctx context.Context, url, destDir string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--quiet", url, destDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %s", string(out))
	}
	return err
}

var noRedirectClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 {
			return http.ErrUseLastResponse
		}
		return nil
	},
	Timeout: 30 * time.Second,
}

func fetchLatestReleaseAssets(ctx context.Context, owner, repo, destDir string) error {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")
	httpReq.Header.Set("User-Agent", "lysis-scanner")

	resp, err := noRedirectClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 304 {
		return nil
	}
	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		return fmt.Errorf("github API rate limited, try again later")
	}
	if resp.StatusCode == 404 {
		return fmt.Errorf("no releases found")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("github API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("parse release: %w", err)
	}

	for _, asset := range release.Assets {
		safeName := filepath.Base(asset.Name)
		assetPath := filepath.Join(destDir, safeName)
		if !strings.HasPrefix(filepath.Clean(assetPath), filepath.Clean(destDir)+string(filepath.Separator)) {
			continue
		}
		if err := downloadAsset(ctx, asset.BrowserDownloadURL, assetPath); err != nil {
			log.Printf("[github] failed to download asset %s: %v", safeName, err)
			continue
		}
	}

	return nil
}

func downloadAsset(ctx context.Context, url, destPath string) error {
	parsedURL, err := parseURL(url)
	if err != nil || !isAllowedHost(parsedURL) {
		return fmt.Errorf("blocked download from %s", url)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("User-Agent", "lysis-scanner")

	resp, err := noRedirectClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, io.LimitReader(resp.Body, 100*1024*1024))
	return err
}

func parseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	return u, nil
}

var allowedHosts = []string{
	"github.com",
	"github-production-release-assist-2a52becb.s3.amazonaws.com",
	"github-releases.githubusercontent.com",
	"objects.githubusercontent.com",
}

func isAllowedHost(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	for _, allowed := range allowedHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return !ip.IsPrivate()
	}
	return false
}

func createZipFromDir(srcDir, destZip string) error {
	zipFile, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), ".git") {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		zf, err := w.Create(relPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(zf, file)
		return err
	})
}
