//go:build windows

package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// discordRPC publishes "Playing <version>" to a running Discord client via
// its local IPC pipe. Every failure is silent: no Discord, no problem.
type discordRPC struct {
	mu   sync.Mutex
	pipe *os.File
}

var discord discordRPC

func (d *discordRPC) frame(op uint32, payload any) []byte {
	body, _ := json.Marshal(payload)
	buf := make([]byte, 8+len(body))
	binary.LittleEndian.PutUint32(buf[0:], op)
	binary.LittleEndian.PutUint32(buf[4:], uint32(len(body)))
	copy(buf[8:], body)
	return buf
}

func (d *discordRPC) connect(appID string) bool {
	for i := 0; i < 10; i++ {
		f, err := os.OpenFile(fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i), os.O_RDWR, 0)
		if err != nil {
			continue
		}
		d.pipe = f
		// handshake
		if _, err := f.Write(d.frame(0, map[string]any{"v": 1, "client_id": appID})); err != nil {
			f.Close()
			d.pipe = nil
			continue
		}
		// Wait briefly for READY. Named pipes don't reliably honour
		// SetReadDeadline on Windows, so bound the wait with a select
		// rather than blocking this goroutine (and the mutex) forever.
		done := make(chan struct{})
		go func() {
			defer close(done)
			hdr := make([]byte, 8)
			if _, err := io.ReadFull(f, hdr); err != nil {
				return
			}
			if n := binary.LittleEndian.Uint32(hdr[4:]); n > 0 && n < 1<<16 {
				io.ReadFull(f, make([]byte, n))
			}
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		return true
	}
	return false
}

// Start publishes the activity; call from any goroutine.
func (d *discordRPC) Start(appID, details, state string) {
	if appID == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pipe == nil && !d.connect(appID) {
		return
	}
	payload := map[string]any{
		"cmd":   "SET_ACTIVITY",
		"nonce": fmt.Sprintf("%d", time.Now().UnixNano()),
		"args": map[string]any{
			"pid": os.Getpid(),
			"activity": map[string]any{
				"details":    details,
				"state":      state,
				"timestamps": map[string]any{"start": time.Now().Unix()},
				"assets": map[string]any{
					"large_image": "lemv",
					"large_text":  "LEMV Launcher",
				},
			},
		},
	}
	if _, err := d.pipe.Write(d.frame(1, payload)); err != nil {
		d.pipe.Close()
		d.pipe = nil
	}
}

// Stop clears the activity.
func (d *discordRPC) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pipe == nil {
		return
	}
	payload := map[string]any{
		"cmd":   "SET_ACTIVITY",
		"nonce": fmt.Sprintf("%d", time.Now().UnixNano()),
		"args":  map[string]any{"pid": os.Getpid()},
	}
	if _, err := d.pipe.Write(d.frame(1, payload)); err != nil {
		d.pipe.Close()
		d.pipe = nil
	}
}
