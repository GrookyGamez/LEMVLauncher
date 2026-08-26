package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const userAgent = "LEMVLauncher/" + launcherVersion

var httpClient = &http.Client{
	Timeout: 15 * time.Minute,
	Transport: &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 32,
		IdleConnTimeout:     90 * time.Second,
	},
}

func httpGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return resp, nil
}

// fetchBytes downloads a URL into memory, retrying a couple of times.
func fetchBytes(url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 700 * time.Millisecond)
		}
		resp, err := httpGet(url)
		if err != nil {
			lastErr = err
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		return data, nil
	}
	return nil, lastErr
}

// fetchBytesAny tries several mirror URLs in order.
func fetchBytesAny(urls []string) ([]byte, error) {
	var errs []string
	for _, u := range urls {
		data, err := fetchBytes(u)
		if err == nil {
			return data, nil
		}
		errs = append(errs, err.Error())
	}
	return nil, errors.New(strings.Join(errs, "; "))
}

func fetchJSON(url string, v any) error {
	data, err := fetchBytes(url)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func fileSHA1(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

var partCounter int64

// downloadFile fetches url into dest unless dest already exists with the
// expected size (or, when no size is known, the expected sha1).
func downloadFile(url, dest, wantSHA1 string, wantSize int64) error {
	return downloadFileProgress(url, dest, wantSHA1, wantSize, nil)
}

// downloadFileProgress is downloadFile with a byte-level progress callback.
func downloadFileProgress(url, dest, wantSHA1 string, wantSize int64, prog func(done, total int64)) error {
	if st, err := os.Stat(dest); err == nil && st.Mode().IsRegular() {
		switch {
		case wantSize > 0 && st.Size() == wantSize:
			return nil
		case wantSize <= 0 && wantSHA1 == "":
			return nil
		case wantSize <= 0:
			if h, err := fileSHA1(dest); err == nil && strings.EqualFold(h, wantSHA1) {
				return nil
			}
		}
	}
	if url == "" {
		return fmt.Errorf("no download URL for %s", filepath.Base(dest))
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		lastErr = downloadOnce(url, dest, wantSHA1, wantSize, prog)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func downloadOnce(url, dest, wantSHA1 string, wantSize int64, prog func(done, total int64)) error {
	resp, err := httpGet(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	total := wantSize
	if total <= 0 {
		total = resp.ContentLength
	}
	tmp := fmt.Sprintf("%s.%d.part", dest, atomic.AddInt64(&partCounter, 1))
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha1.New()
	var src io.Reader = resp.Body
	if prog != nil {
		src = &progressReader{r: resp.Body, total: total, report: prog}
	}
	n, err := io.Copy(io.MultiWriter(f, h), src)
	f.Close()
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if wantSize > 0 && n != wantSize {
		os.Remove(tmp)
		return fmt.Errorf("size mismatch for %s (got %d bytes, expected %d)", url, n, wantSize)
	}
	if wantSHA1 != "" && !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), wantSHA1) {
		os.Remove(tmp)
		return fmt.Errorf("checksum mismatch for %s", url)
	}
	os.Remove(dest)
	return os.Rename(tmp, dest)
}

// runParallel runs tasks on a small worker pool. onDone is called after each
// task finishes (from worker goroutines). The first error stops the pool.
func runParallel(workers int, tasks []func() error, onDone func(done, total int)) error {
	if len(tasks) == 0 {
		return nil
	}
	if workers > len(tasks) {
		workers = len(tasks)
	}
	var (
		next     int64 = -1
		done     int64
		firstErr error
		mu       sync.Mutex
		wg       sync.WaitGroup
		stop     atomic.Bool
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				i := int(atomic.AddInt64(&next, 1))
				if i >= len(tasks) {
					return
				}
				if err := tasks[i](); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					stop.Store(true)
					return
				}
				d := int(atomic.AddInt64(&done, 1))
				if onDone != nil {
					onDone(d, len(tasks))
				}
			}
		}()
	}
	wg.Wait()
	return firstErr
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

// copyFile copies src to dst unless dst already has the same size.
func copyFile(src, dst string) error {
	ss, err := os.Stat(src)
	if err != nil {
		return err
	}
	if ds, err := os.Stat(dst); err == nil && ds.Size() == ss.Size() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	os.Remove(dst)
	return os.Rename(tmp, dst)
}

// progressReader reports download progress roughly every 256 KiB.
type progressReader struct {
	r           io.Reader
	done, total int64
	lastReport  int64
	report      func(done, total int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	if p.done-p.lastReport >= 256*1024 || err == io.EOF {
		p.lastReport = p.done
		p.report(p.done, p.total)
	}
	return n, err
}

func mib(n int64) string {
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}
