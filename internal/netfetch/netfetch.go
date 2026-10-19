// Package netfetch is the ONLY package allowed to touch the network
// (invariant S11, enforced by scripts/check-boundaries.sh). It is used solely
// by commands the user runs explicitly, such as `thine fonts install`, and it
// verifies every download against a pinned SHA-256.
package netfetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// MaxBytes caps any single download.
const MaxBytes = 256 << 20

// Progress reports bytes received so far and the expected total (may be -1).
type Progress func(done, total int64)

// Download fetches url into dest and verifies its SHA-256 before returning.
// On any failure dest is removed, so an unverified file never lingers.
func Download(ctx context.Context, url, dest, wantSHA256 string, progress Progress) (err error) {
	if wantSHA256 == "" {
		return errors.New("refusing an unpinned download (no SHA-256)")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "thine (+https://github.com/YourAKShaw/.thinerc)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(dest)
		}
	}()
	h := sha256.New()
	var done int64
	buf := make([]byte, 64<<10)
	body := io.LimitReader(resp.Body, MaxBytes+1)
	for {
		n, rerr := body.Read(buf)
		if n > 0 {
			done += int64(n)
			if done > MaxBytes {
				return fmt.Errorf("download exceeds %d MiB", MaxBytes>>20)
			}
			h.Write(buf[:n])
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			if progress != nil {
				progress(done, resp.ContentLength)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != wantSHA256 {
		return fmt.Errorf("checksum mismatch: got %s, want %s — the download was discarded", got, wantSHA256)
	}
	return f.Sync()
}
