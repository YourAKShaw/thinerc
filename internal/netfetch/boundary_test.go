package netfetch

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestS11_OnlyNetfetchTouchesTheNetwork walks the module and fails if any
// other package imports net/http (no hidden network calls, no telemetry).
func TestS11_OnlyNetfetchTouchesTheNetwork(t *testing.T) {
	root := filepath.Join("..", "..")
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		if strings.Contains(filepath.ToSlash(p), "internal/netfetch/") {
			return nil
		}
		b, _ := os.ReadFile(p)
		if strings.Contains(string(b), `"net/http"`) {
			t.Errorf("%s imports net/http; only internal/netfetch may", p)
		}
		return nil
	})
}
