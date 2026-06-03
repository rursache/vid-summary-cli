package model

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func download(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: unexpected status %s", url, resp.Status)
	}

	partial := dest + ".partial"
	out, err := os.Create(partial)
	if err != nil {
		return err
	}

	src := io.TeeReader(resp.Body, &progress{total: resp.ContentLength})
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		_ = os.Remove(partial)
		return fmt.Errorf("write model: %w", err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(partial)
		return err
	}
	fmt.Fprintln(os.Stderr)
	return os.Rename(partial, dest)
}

type progress struct {
	total   int64
	written int64
	lastPct int
}

func (p *progress) Write(b []byte) (int, error) {
	n := len(b)
	p.written += int64(n)
	if p.total <= 0 {
		return n, nil
	}
	pct := int(p.written * 100 / p.total)
	if pct != p.lastPct {
		p.lastPct = pct
		fmt.Fprintf(os.Stderr, "\r  %3d%%  %d/%d MB", pct, p.written>>20, p.total>>20)
	}
	return n, nil
}
