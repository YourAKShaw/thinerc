//go:build darwin

package ports

import (
	"context"
	"time"

	"github.com/YourAKShaw/thinerc/internal/platform"
)

// All returns every listening TCP socket using lsof's machine-readable -F
// output (one subprocess; it exits 1 when nothing listens).
func All(ctx context.Context) ([]Listener, error) {
	r, err := platform.Run(ctx, 5*time.Second, "lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pcutn")
	if err != nil && len(r.Stdout) == 0 {
		if r.ExitCode == 1 {
			return nil, nil
		}
		return nil, err
	}
	return dedupe(parseLsofF(string(r.Stdout))), nil
}
