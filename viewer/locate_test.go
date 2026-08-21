package viewer

import (
	"os/exec"
	"strings"
	"testing"
)

// skipIfTmuxHasNvim skips a test if a real `tmux show-environment NVIM`
// would find something in the environment the test happens to run in.
// LocateNvim's tmux fallback runs unconditionally (per implementation/
// VIEWER.md's Locating Neovim section, no interface was introduced to make
// it mockable, since three fixed steps didn't seem to warrant one) — so a
// test asserting "the flag was used" or "nothing was found" would
// otherwise depend on ambient state outside the test's control. This is
// not hypothetical: this project's own dev session is launched via `nv`,
// which sets tmux's session-scoped $NVIM — so running these tests from
// inside that session would otherwise fail on exactly the environment
// they were written in.
func skipIfTmuxHasNvim(t *testing.T) {
	t.Helper()
	out, err := exec.Command("tmux", "show-environment", "NVIM").Output()
	if err == nil && strings.HasPrefix(strings.TrimSpace(string(out)), "NVIM=") {
		t.Skip("skipping: this environment's tmux session already has $NVIM set, which would win over what this test is trying to isolate")
	}
}

func TestLocateNvim_ProcessEnvWins(t *testing.T) {
	t.Setenv("NVIM", "/tmp/from-env.sock")

	addr, err := LocateNvim("/tmp/from-flag.sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "/tmp/from-env.sock" {
		t.Errorf("expected the process env $NVIM to win over the flag, got %q", addr)
	}
}

func TestLocateNvim_FlagUsedWhenNothingElsePresent(t *testing.T) {
	skipIfTmuxHasNvim(t)
	t.Setenv("NVIM", "")

	addr, err := LocateNvim("/tmp/from-flag.sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "/tmp/from-flag.sock" {
		t.Errorf("expected the --nvim flag to be used, got %q", addr)
	}
}

func TestLocateNvim_NoSourceReturnsErrNoNvimSession(t *testing.T) {
	skipIfTmuxHasNvim(t)
	t.Setenv("NVIM", "")

	if _, err := LocateNvim(""); err != ErrNoNvimSession {
		t.Errorf("expected ErrNoNvimSession when nothing is set, got %v", err)
	}
}
