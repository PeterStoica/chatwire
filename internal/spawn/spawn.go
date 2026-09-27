package spawn

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func Detached(ctx context.Context, executable string, args []string, logPath string) (int, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return 0, fmt.Errorf("spawn: log: %w", err)
	}
	defer logFile.Close()
	cmd := exec.CommandContext(context.WithoutCancel(ctx), executable, args...)
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = detachedAttributes()
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("spawn: start %s: %w", executable, err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Process.Release(); err != nil {
		return 0, fmt.Errorf("spawn: release: %w", err)
	}
	return pid, nil
}
