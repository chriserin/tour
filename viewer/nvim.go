package viewer

import (
	_ "embed"
	"fmt"
	"path/filepath"

	"github.com/neovim/go-client/nvim"
)

// NvimClient jumps a running Neovim to a location. Defined as an interface
// so the viewer's navigation logic can be tested against a fake instead of
// a real RPC connection.
type NvimClient interface {
	// Jump opens file in Neovim's currently active window (reusing an
	// already-open buffer for it, rather than reloading) and moves the
	// cursor to line. file must be absolute — see implementation/VIEWER.md's
	// Jump target section for why: Neovim's own working directory is
	// unrelated to the tour's, so a relative path would resolve against the
	// wrong one.
	//
	// scroll is the number of lines of context to leave visible above line,
	// per design/SCROLL.md. A non-nil scroll also pins the window's
	// 'scrolloff' to 0 for the current window and briefly suppresses
	// CursorMoved/CursorMovedI/BufEnter (via 'eventignore') so a
	// reviewer's own cursor-centering autocmds don't fight the jump (see
	// jumpLua's comment for why); a nil scroll clears the 'scrolloff'
	// override, restoring whatever the reviewer's own config set, matching
	// Jump's behavior before this parameter existed.
	Jump(file string, line int, scroll *int) error
}

type realNvimClient struct {
	nv *nvim.Nvim
}

// ConnectNvim dials a running Neovim at addr.
func ConnectNvim(addr string) (NvimClient, error) {
	nv, err := nvim.Dial(addr)
	if err != nil {
		return nil, err
	}
	return &realNvimClient{nv: nv}, nil
}

// jumpLua is the Lua chunk that performs the jump — see jump.lua's own
// comment for the full mechanics and why each piece of it exists (the
// 'scrolloff' and 'eventignore' handling in particular). Embedded from a
// real .lua file rather than a Go string constant so it can be edited,
// diffed, and read as actual Lua — with real comments and syntax
// highlighting — rather than an escaped string literal, the same
// reasoning agentinstructions/instructions.md uses for its embedded
// prose.
//
//go:embed jump.lua
var jumpLua string

func (c *realNvimClient) Jump(file string, line int, scroll *int) error {
	if !filepath.IsAbs(file) {
		return fmt.Errorf("internal error: jump target must be absolute, got %q", file)
	}
	return c.nv.ExecLua(jumpLua, nil, file, line, scroll)
}
