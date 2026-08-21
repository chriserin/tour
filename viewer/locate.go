package viewer

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// ErrNoNvimSession means none of the three conventions in
// design/VIEWER.md's Locating Neovim section found an address.
var ErrNoNvimSession = errors.New("no Neovim session found")

// LocateNvim finds the address of a running Neovim to connect to, checking,
// in order (per design/VIEWER.md's Locating Neovim section):
//
//  1. $NVIM in the process environment — set automatically by Neovim's own
//     :terminal for its direct children.
//  2. $NVIM in the tmux session environment — set by a launcher (`nv`) that
//     starts Neovim listening on a session-scoped socket and records that
//     path with `tmux setenv`. Read via `tmux show-environment NVIM` rather
//     than re-deriving the socket path, so this doesn't need to know the
//     launcher's own naming convention.
//  3. flagAddr, the --nvim flag's value, if non-empty.
//
// Steps 1 and 2 share the env var name deliberately: a pane that's
// Neovim's own :terminal already has $NVIM in its inherited process
// environment, so it wins without any explicit precedence logic.
func LocateNvim(flagAddr string) (string, error) {
	if addr := os.Getenv("NVIM"); addr != "" {
		return addr, nil
	}
	if addr, ok := tmuxNvimEnv(); ok {
		return addr, nil
	}
	if flagAddr != "" {
		return flagAddr, nil
	}
	return "", ErrNoNvimSession
}

func tmuxNvimEnv() (string, bool) {
	out, err := exec.Command("tmux", "show-environment", "NVIM").Output()
	if err != nil {
		// No tmux, no session, or NVIM unset in it — all treated the same:
		// fall through to the next source, not an error.
		return "", false
	}
	addr, ok := strings.CutPrefix(strings.TrimSpace(string(out)), "NVIM=")
	if !ok || addr == "" {
		return "", false
	}
	return addr, true
}
