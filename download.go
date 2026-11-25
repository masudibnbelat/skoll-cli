package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// downloadFileProgress downloads url to destPath with a live progress
// line (percent + MB) on a terminal, or plain lines when piped. It
// skips the download when the file is already there, and downloads
// to a temp file first, so a killed download never leaves behind a
// file that looks complete.
//
// A download that was interrupted resumes where it stopped: the
// half-downloaded .tmp file is kept, and the next run asks the
// server for the rest with a Range request (servers that do not
// support it get a fresh download from the start). Re-running any
// command that downloads is safe — nothing already fetched is
// fetched twice.
//
// Before downloading, it checks there is enough free disk space
// for the remaining bytes (when the server reports a size); a full
// disk fails here with a clear message instead of halfway through.
func downloadFileProgress(url, destPath, label string) error {
	if _, err := os.Stat(destPath); err == nil {
		return nil
	}
	tmpPath := destPath + ".tmp"
	var resumeFrom int64
	if st, err := os.Stat(tmpPath); err == nil && st.Size() > 0 {
		resumeFrom = st.Size()
	}
	client := &http.Client{
		Timeout: 30 * time.Minute,
		Transport: &http.Transport{
			Proxy:              http.ProxyFromEnvironment,
			DisableCompression: true,
		},
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if resumeFrom > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeFrom))
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	if resp.StatusCode == 200 && resumeFrom > 0 {
		// The server does not support Range: start over, so the
		// old half-file does not corrupt the new download.
		resumeFrom = 0
	}
	total := resp.ContentLength
	if resp.StatusCode == 206 {
		// ContentLength is only the remaining bytes here.
		total += resumeFrom
	}
	if err := checkDiskSpace(destPath, total-resumeFrom); err != nil {
		return err
	}
	var f *os.File
	if resumeFrom > 0 {
		f, err = os.OpenFile(tmpPath, os.O_WRONLY|os.O_APPEND, 0o644)
	} else {
		f, err = os.Create(tmpPath)
	}
	if err != nil {
		return err
	}
	terminal := isTerminal(os.Stdout)
	start := time.Now()
	if !terminal {
		// No terminal: plain lines, no live bar.
		if resumeFrom > 0 {
			say("Resuming " + label + " (" + formatBytes(resumeFrom) + " already downloaded) ...")
		} else {
			say("Downloading " + label + " ...")
		}
		n, err := io.Copy(f, resp.Body)
		f.Close()
		if err != nil {
			// Keep the .tmp file: the next run resumes from it.
			return err
		}
		if err := os.Rename(tmpPath, destPath); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		say(label + " downloaded (" + formatBytes(resumeFrom+n) + " in " + formatDuration(time.Since(start)) + ")")
		return nil
	}
	// Terminal: live line on one line — a bar with percent, sizes,
	// speed and ETA when the size is known, size + speed when not.
	if resumeFrom > 0 {
		stepLine("Resuming " + label + " (" + formatBytes(resumeFrom) + " already downloaded)")
	} else {
		stepLine("Downloading " + label)
	}
	bar := newDownloadBar(label, total)
	var written = resumeFrom
	var mu sync.Mutex
	counter := &countingReader{r: resp.Body, n: &written, mu: &mu}
	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				mu.Lock()
				w := written
				mu.Unlock()
				bar.render(w)
			}
		}
	}()
	n, err := io.Copy(f, counter)
	close(done)
	f.Close()
	if err != nil {
		fmt.Println()
		// Keep the .tmp file: the next run resumes from it.
		return err
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	bar.finish(resumeFrom + n)
	return nil
}

// checkDiskSpace reports whether the disk holding destPath has room
// for need more bytes (plus a small margin, so the disk is not
// filled to the last byte). It passes silently when the size is
// unknown (need <= 0) or the free space cannot be read (Windows
// stub, or an exotic filesystem).
func checkDiskSpace(destPath string, need int64) error {
	if need <= 0 {
		return nil
	}
	free, ok := freeDiskSpace(destPath)
	if !ok {
		return nil
	}
	// 10 MB margin: a download that exactly fills the disk fails
	// on the last write.
	need += 10 * 1024 * 1024
	if uint64(need) > free {
		return fmt.Errorf("not enough disk space: %s needs %s more, but only %s is free on this disk — free some space and re-run (the download resumes where it stopped)", destPath, formatBytes(need), formatBytes(int64(free)))
	}
	return nil
}

type countingReader struct {
	r  io.Reader
	n  *int64
	mu *sync.Mutex
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.mu.Lock()
	*c.n += int64(n)
	c.mu.Unlock()
	return n, err
}
