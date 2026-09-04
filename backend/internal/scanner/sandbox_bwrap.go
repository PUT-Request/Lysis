//go:build linux

package scanner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type bwrapSandbox struct {
	sandboxImpl
}

func NewBwrapSandbox(scanID string, cfg SandboxConfig) (Sandbox, error) {
	if _, err := os.Stat(cfg.BwrapPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("bwrap not found at %s", cfg.BwrapPath)
	}

	for _, tool := range cfg.Tools {
		if _, err := exec.LookPath(tool); err != nil {
			return nil, fmt.Errorf("required tool not found: %s", tool)
		}
	}

	if err := os.MkdirAll(cfg.TempDir, 0755); err != nil {
		return nil, fmt.Errorf("create temp root: %w", err)
	}

	return &bwrapSandbox{
		sandboxImpl: sandboxImpl{
			scanID:  scanID,
			cfg:     cfg,
			workDir: "/work",
		},
	}, nil
}

func (s *bwrapSandbox) RunCommand(ctx context.Context, cmd string, args ...string) (string, string, error) {
	timeout := time.Duration(s.cfg.CommandTimeoutSecs) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var fullArgs []string
	fullArgs = append(fullArgs,
		"--unshare-all",
		"--new-session",
		"--die-with-parent",
		"--hostname", "sandbox",
		"--setenv", "PATH", "/usr/bin:/bin",
		"--setenv", "HOME", "/work",
		"--ro-bind", "/usr/bin", "/usr/bin",
		"--ro-bind", "/usr/lib", "/usr/lib",
		"--ro-bind", "/usr/lib64", "/usr/lib64",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind", "/lib64", "/lib64",
		"--ro-bind", "/usr/share", "/usr/share",
		"--ro-bind", "/etc/alternatives", "/etc/alternatives",
		"--ro-bind", "/etc/ld.so.cache", "/etc/ld.so.cache",
		"--ro-bind", s.hostPath, "/input",
	)

	fullArgs = append(fullArgs, "--tmpfs", "/work")
	fullArgs = append(fullArgs, "--tmpfs", "/tmp")
	fullArgs = append(fullArgs, "--tmpfs", "/run")

	fullArgs = append(fullArgs,
		"--dev-bind", "/dev/null", "/dev/null",
		"--dev-bind", "/dev/zero", "/dev/zero",
		"--dev-bind", "/dev/random", "/dev/random",
		"--dev-bind", "/dev/urandom", "/dev/urandom",
	)

	fullArgs = append(fullArgs,
		"--proc", "/proc",
		"--chdir", "/work",
		"--",
		"sh", "-c",
		strings.Join(append([]string{cmd}, args...), " "),
	)

	exe := exec.CommandContext(ctx, s.cfg.BwrapPath, fullArgs...)
	exe.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	exe.Stdout = stdout
	exe.Stderr = stderr

	err := exe.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return stdout.String(), stderr.String(), fmt.Errorf("command timed out after %v", timeout)
	}

	return stdout.String(), stderr.String(), err
}
