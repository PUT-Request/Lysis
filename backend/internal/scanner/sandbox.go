package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Sandbox interface {
	Create(ctx context.Context) error
	RunCommand(ctx context.Context, cmd string, args ...string) (stdout string, stderr string, err error)
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte) error
	WorkDir() string
	HostPath() string
	Destroy() error
}

type SandboxConfig struct {
	TempDir            string
	Tools              []string
	BwrapPath          string
	CommandTimeoutSecs int
	MaxMemoryMB        int
	CleanupOnComplete  bool
}

func NewSandbox(scanID string, cfg SandboxConfig) (Sandbox, error) {
	s := &sandboxImpl{
		scanID: scanID,
		cfg:    cfg,
	}
	if err := os.MkdirAll(cfg.TempDir, 0755); err != nil {
		return nil, fmt.Errorf("create temp root: %w", err)
	}
	return s, nil
}

type sandboxImpl struct {
	scanID   string
	cfg      SandboxConfig
	workDir  string
	hostPath string
}

func (s *sandboxImpl) Create(ctx context.Context) error {
	s.hostPath = fmt.Sprintf("%s/%s", s.cfg.TempDir, s.scanID)
	s.workDir = "/work"

	if err := os.MkdirAll(s.hostPath, 0755); err != nil {
		return fmt.Errorf("create sandbox dir: %w", err)
	}
	return nil
}

func (s *sandboxImpl) RunCommand(ctx context.Context, cmd string, args ...string) (string, string, error) {
	return "", "", fmt.Errorf("sandbox not available on this platform — compile with linux build tag for bwrap")
}

func (s *sandboxImpl) ReadFile(path string) ([]byte, error) {
	safePath := sanitizePath(path)
	fullPath := filepath.Join(s.hostPath, safePath)
	if !strings.HasPrefix(fullPath, filepath.Clean(s.hostPath)+string(filepath.Separator)) {
		return nil, fmt.Errorf("path traversal denied: %s", path)
	}
	return os.ReadFile(fullPath)
}

func (s *sandboxImpl) WriteFile(path string, data []byte) error {
	safePath := sanitizePath(path)
	fullPath := filepath.Join(s.hostPath, safePath)
	if !strings.HasPrefix(fullPath, filepath.Clean(s.hostPath)+string(filepath.Separator)) {
		return fmt.Errorf("path traversal denied: %s", path)
	}
	return os.WriteFile(fullPath, data, 0644)
}

func sanitizePath(path string) string {
	path = filepath.Clean(path)
	if strings.HasPrefix(path, "/") {
		path = strings.TrimPrefix(path, "/input/")
		path = strings.TrimPrefix(path, "/work/")
		path = strings.TrimPrefix(path, "/")
	}
	if strings.HasPrefix(path, "..") {
		path = strings.TrimPrefix(path, "../")
	}
	return path
}

func (s *sandboxImpl) WorkDir() string {
	return s.workDir
}

func (s *sandboxImpl) HostPath() string {
	return s.hostPath
}

func (s *sandboxImpl) Destroy() error {
	if !s.cfg.CleanupOnComplete {
		return nil
	}
	return os.RemoveAll(s.hostPath)
}
