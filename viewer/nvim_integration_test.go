//go:build nvim_integration

// Real Neovim RPC integration tests. Gated behind a build tag (per
// implementation/VIEWER.md's Testing section) since they need a real
// `nvim` binary on PATH — the fast unit test suite shouldn't depend on
// that. Run with: go test -tags nvim_integration ./viewer/...
package viewer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/neovim/go-client/nvim"
)

func startEmbeddedNvim(t *testing.T) (sock string) {
	t.Helper()
	sock = filepath.Join(t.TempDir(), "nvim.sock")

	cmd := exec.Command("nvim", "--headless", "--listen", sock)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start nvim: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sock); err == nil {
			return sock
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("nvim never created its socket at %s", sock)
	return ""
}

func TestNvimIntegration_JumpMovesTheRealCursor(t *testing.T) {
	sock := startEmbeddedNvim(t)

	client, err := ConnectNvim(sock)
	if err != nil {
		t.Fatalf("ConnectNvim: %v", err)
	}

	file := filepath.Join(t.TempDir(), "app.rb")
	if err := os.WriteFile(file, []byte("line1\nline2\nline3\nline4\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := client.Jump(file, 3); err != nil {
		t.Fatalf("Jump: %v", err)
	}

	// Verify directly against the real Neovim, independent of our own
	// client wrapper, that the buffer and cursor actually moved.
	v, err := nvim.Dial(sock)
	if err != nil {
		t.Fatalf("dial for verification: %v", err)
	}
	buf, err := v.CurrentBuffer()
	if err != nil {
		t.Fatal(err)
	}
	name, err := v.BufferName(buf)
	if err != nil {
		t.Fatal(err)
	}
	if name != file {
		t.Errorf("expected current buffer %q, got %q", file, name)
	}
	win, err := v.CurrentWindow()
	if err != nil {
		t.Fatal(err)
	}
	pos, err := v.WindowCursor(win)
	if err != nil {
		t.Fatal(err)
	}
	if pos[0] != 3 {
		t.Errorf("expected cursor on line 3, got line %d", pos[0])
	}
}

func TestNvimIntegration_JumpReusesAlreadyOpenBuffer(t *testing.T) {
	sock := startEmbeddedNvim(t)
	client, err := ConnectNvim(sock)
	if err != nil {
		t.Fatalf("ConnectNvim: %v", err)
	}

	file := filepath.Join(t.TempDir(), "app.rb")
	if err := os.WriteFile(file, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := client.Jump(file, 1); err != nil {
		t.Fatalf("first Jump: %v", err)
	}
	if err := client.Jump(file, 2); err != nil {
		t.Fatalf("second Jump: %v", err)
	}

	v, err := nvim.Dial(sock)
	if err != nil {
		t.Fatalf("dial for verification: %v", err)
	}
	bufs, err := v.Buffers()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, b := range bufs {
		name, err := v.BufferName(b)
		if err != nil {
			t.Fatal(err)
		}
		if name == file {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one buffer for %q (reused, not reopened), found %d", file, count)
	}
}
