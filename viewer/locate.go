package viewer

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// ErrNoNvimSession means none of the conventions in
// design/VIEWER.md's Locating Neovim section found an address.
var ErrNoNvimSession = errors.New("no Neovim session found")

// LocateNvim finds the address of a running Neovim to connect to, checking,
// in order (per design/VIEWER.md's Locating Neovim section):
//
//  1. $NVIM in the process environment — set automatically by Neovim's own
//     :terminal for its direct children.
//  2. /tmp/nvim-<label>.sock, where <label> is the current Herdr
//     workspace's label — the socket a launcher started Neovim on for this
//     workspace. Only used when $HERDR_WORKSPACE_ID is set and the socket
//     file exists.
//  3. $NVIM in the tmux session environment — set by a launcher (`nv`) that
//     starts Neovim listening on a session-scoped socket and records that
//     path with `tmux setenv`. Read via `tmux show-environment NVIM` rather
//     than re-deriving the socket path, so this doesn't need to know the
//     launcher's own naming convention.
//  4. flagAddr, the --nvim flag's value, if non-empty.
//
// Steps 1 and 3 share the env var name deliberately: a pane that's
// Neovim's own :terminal already has $NVIM in its inherited process
// environment, so it wins without any explicit precedence logic.
func LocateNvim(flagAddr string) (string, error) {
	if addr := os.Getenv("NVIM"); addr != "" {
		return addr, nil
	}
	if addr, ok := herdrNvimSocket(); ok {
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

func herdrNvimSocket() (string, bool) {
	id := os.Getenv("HERDR_WORKSPACE_ID")
	if id == "" {
		return "", false
	}
	out, err := exec.Command("herdr", "workspace", "get", id).Output()
	if err != nil {
		// Same as tmux: no herdr or no such workspace falls through.
		return "", false
	}
	label, ok := parseHerdrWorkspaceLabel(out)
	if !ok {
		return "", false
	}
	addr := herdrNvimSocketPath(label)
	if _, err := os.Stat(addr); err != nil {
		// No Neovim launched for this workspace yet.
		return "", false
	}
	return addr, true
}

func herdrNvimSocketPath(label string) string {
	return "/tmp/nvim-" + label + ".sock"
}

// parseHerdrWorkspaceLabel extracts the workspace label from
// `herdr workspace get`'s JSON output.
func parseHerdrWorkspaceLabel(out []byte) (string, bool) {
	var resp struct {
		Result struct {
			Workspace struct {
				Label string `json:"label"`
			} `json:"workspace"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", false
	}
	label := resp.Result.Workspace.Label
	return label, label != ""
}
