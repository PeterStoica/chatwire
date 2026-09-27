package qrpage

import (
	"context"
	"os/exec"
)

func Open(ctx context.Context, url string) error {
	return exec.CommandContext(ctx, "open", url).Run()
}
