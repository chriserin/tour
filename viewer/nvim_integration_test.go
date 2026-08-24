//go:build nvim_integration

// Real Neovim RPC integration tests. Gated behind a build tag (per
// implementation/VIEWER.md's Testing section) since they need a real
// `nvim` binary on PATH — the fast unit test suite shouldn't depend on
// that. Run with: go test -tags nvim_integration ./viewer/...
package viewer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	if err := client.Jump(file, 3, nil); err != nil {
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

	if err := client.Jump(file, 1, nil); err != nil {
		t.Fatalf("first Jump: %v", err)
	}
	if err := client.Jump(file, 2, nil); err != nil {
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

func topline(t *testing.T, v *nvim.Nvim) int {
	t.Helper()
	var view map[string]int
	if err := v.Call("winsaveview", &view); err != nil {
		t.Fatal(err)
	}
	tl, ok := view["topline"]
	if !ok {
		t.Fatalf("expected winsaveview's result to have a topline field, got %v", view)
	}
	return tl
}

// @ft:39, @ft:41
func TestNvimIntegration_JumpWithScrollOffsetPositionsTheWindow(t *testing.T) {
	sock := startEmbeddedNvim(t)
	client, err := ConnectNvim(sock)
	if err != nil {
		t.Fatalf("ConnectNvim: %v", err)
	}

	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i+1)
	}
	file := filepath.Join(t.TempDir(), "app.rb")
	if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	scroll := 4
	if err := client.Jump(file, 42, &scroll); err != nil {
		t.Fatalf("Jump: %v", err)
	}

	v, err := nvim.Dial(sock)
	if err != nil {
		t.Fatalf("dial for verification: %v", err)
	}
	if got, want := topline(t, v), 38; got != want {
		t.Errorf("expected topline %d (line 42 - scroll 4), got %d", want, got)
	}
}

// @ft:41
func TestNvimIntegration_ScrollOffsetLargerThanLineClampsToTop(t *testing.T) {
	sock := startEmbeddedNvim(t)
	client, err := ConnectNvim(sock)
	if err != nil {
		t.Fatalf("ConnectNvim: %v", err)
	}

	file := filepath.Join(t.TempDir(), "app.rb")
	if err := os.WriteFile(file, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	scroll := 10
	if err := client.Jump(file, 3, &scroll); err != nil {
		t.Fatalf("Jump: %v", err)
	}

	v, err := nvim.Dial(sock)
	if err != nil {
		t.Fatalf("dial for verification: %v", err)
	}
	if got, want := topline(t, v), 1; got != want {
		t.Errorf("expected topline to clamp to %d, got %d", want, got)
	}
}

// TestNvimIntegration_ScrollOffsetHoldsAgainstHighScrolloff guards against a
// regression that a fake Neovim can't catch: 'scrolloff' isn't a one-time
// clamp on cursor movement, it's an invariant Neovim re-enforces on every
// redraw for the life of the window, so a reviewer with a large 'scrolloff'
// (a common "always center the cursor" trick — plausibly what this whole
// feature was motivated by) would see winrestview's topline snap straight
// back to centered the moment any redraw happens, unless jumpLua also pins
// 'scrolloff' itself. Verified against a real Neovim, not fakeNvim, because
// this is exactly the kind of behavior a fake can't exhibit: it doesn't
// have a redraw cycle to reassert anything against.
func TestNvimIntegration_ScrollOffsetHoldsAgainstHighScrolloff(t *testing.T) {
	sock := startEmbeddedNvim(t)
	v, err := nvim.Dial(sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	// A high 'scrolloff' is what makes Neovim insist on centering the
	// cursor — the exact configuration this test needs to jump despite.
	if err := v.Command("set scrolloff=999"); err != nil {
		t.Fatalf("set scrolloff: %v", err)
	}

	client, err := ConnectNvim(sock)
	if err != nil {
		t.Fatalf("ConnectNvim: %v", err)
	}

	lines := make([]string, 200)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i+1)
	}
	file := filepath.Join(t.TempDir(), "app.rb")
	if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	scroll := 1
	if err := client.Jump(file, 103, &scroll); err != nil {
		t.Fatalf("Jump: %v", err)
	}

	if got, want := topline(t, v), 102; got != want {
		t.Errorf("expected topline %d (line 103 - scroll 1) despite scrolloff=999, got %d", want, got)
	}
}

// TestNvimIntegration_NoScrollOffsetRestoresScrolloffAfterScrollJump checks
// the other half of jumpLua's 'scrolloff' handling: a slide with no :scroll
// following one that had it must not stay pinned to the scrolloff=0
// override from the earlier jump — the reviewer's own 'scrolloff' should
// govern again once a slide stops asking for an explicit position.
func TestNvimIntegration_NoScrollOffsetRestoresScrolloffAfterScrollJump(t *testing.T) {
	sock := startEmbeddedNvim(t)
	v, err := nvim.Dial(sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := v.Command("set scrolloff=999"); err != nil {
		t.Fatalf("set scrolloff: %v", err)
	}

	client, err := ConnectNvim(sock)
	if err != nil {
		t.Fatalf("ConnectNvim: %v", err)
	}

	lines := make([]string, 200)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i+1)
	}
	file := filepath.Join(t.TempDir(), "app.rb")
	if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	scroll := 1
	if err := client.Jump(file, 103, &scroll); err != nil {
		t.Fatalf("first Jump: %v", err)
	}
	if err := client.Jump(file, 150, nil); err != nil {
		t.Fatalf("second Jump: %v", err)
	}

	// Check the *effective* 'scrolloff' (not just the window-local raw
	// value, which reports -1 once cleared rather than the value it falls
	// back to) — this is what actually governs scrolling, and is the
	// direct signal that the override was cleared rather than an inferred
	// one from topline math, which a "scroll into view" cursor move can
	// satisfy in more than one way even with a leftover override.
	var effective int
	if err := v.Eval("&scrolloff", &effective); err != nil {
		t.Fatalf("eval &scrolloff: %v", err)
	}
	if effective != 999 {
		t.Errorf("expected the reviewer's own 'scrolloff' (999) to govern again after a no-scroll jump, got effective 'scrolloff' %d", effective)
	}
}

// TestNvimIntegration_ScrollOffsetHoldsAgainstCursorCenteringAutocmd guards
// against the other way a reviewer's config can defeat winrestview: not
// 'scrolloff', but a CursorMoved/BufEnter autocommand that recenters
// directly (the exact shape of a real "keep the cursor centered" plugin
// found in the wild while testing this feature manually). Such an autocmd
// fires once Neovim returns to its main loop after jumpLua finishes —
// measured at roughly 50-300ms later against a real Neovim, not on the
// same tick — so a single vim.schedule'd re-application isn't reliably
// late enough to come after it; jumpLua also needs the later
// vim.defer_fn(pin, 50) for this case to actually be covered. This can't
// be caught by fakeNvim (there's no autocmd/event system or redraw cycle
// to simulate), so it's verified here against a real Neovim.
//
// CursorMoved specifically needs an attached UI to fire at all — a plain
// `--headless` instance with nothing attached never triggers it, even
// after waiting, since there's no redraw cycle checking for it. Every
// other test in this file uses startEmbeddedNvim's bare `--headless`
// process, which would make this test pass unconditionally regardless of
// the fix (the autocmd installed below would simply never run), so this
// one attaches a minimal fake UI instead, matching how the bug was
// actually found.
func TestNvimIntegration_ScrollOffsetHoldsAgainstCursorCenteringAutocmd(t *testing.T) {
	sock := startEmbeddedNvim(t)
	v, err := nvim.Dial(sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	// The "redraw" UI notification stream isn't consumed by this test;
	// register a no-op handler so go-client doesn't log a warning for
	// every frame instead of leaving it unregistered.
	if err := v.RegisterHandler("redraw", func(...interface{}) {}); err != nil {
		t.Fatalf("register redraw handler: %v", err)
	}
	if err := v.AttachUI(80, 24, map[string]interface{}{}); err != nil {
		t.Fatalf("AttachUI: %v", err)
	}

	recenterAutocmd := `vim.api.nvim_create_autocmd("CursorMoved", {
  callback = function() vim.cmd("normal! zz") end,
})`
	if err := v.ExecLua(recenterAutocmd, nil); err != nil {
		t.Fatalf("install recentering autocmd: %v", err)
	}

	client, err := ConnectNvim(sock)
	if err != nil {
		t.Fatalf("ConnectNvim: %v", err)
	}

	lines := make([]string, 200)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i+1)
	}
	file := filepath.Join(t.TempDir(), "app.rb")
	if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	scroll := 1
	if err := client.Jump(file, 103, &scroll); err != nil {
		t.Fatalf("Jump: %v", err)
	}

	// Both the autocmd's CursorMoved firing and jumpLua's own vim.schedule
	// callback are deferred to a later main-loop iteration, not run by the
	// time Jump's RPC call returns (confirmed directly: an equivalent
	// CursorMoved autocmd installed the same way took ~300ms to fire
	// against this same AttachUI setup). Reading topline immediately after
	// Jump would pass regardless of the fix, simply because neither one
	// has run yet — this waits long enough for both to have settled, the
	// same way a real terminal's redraw cycle would before a reviewer ever
	// sees the frame.
	time.Sleep(500 * time.Millisecond)
	if got, want := topline(t, v), 102; got != want {
		t.Errorf("expected topline %d (line 103 - scroll 1) despite a cursor-centering autocmd, got %d", want, got)
	}
}
