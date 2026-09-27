package qrpage

import (
	"context"
	"os/exec"
)

func Open(ctx context.Context, url string) error {
	return exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url).Run()
}
