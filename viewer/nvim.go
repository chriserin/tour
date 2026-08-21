package viewer

import (
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
	Jump(file string, line int) error
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

// jumpLua takes file and line as Lua varargs (`...`) rather than being
// string-interpolated into the script — passing them as real msgpack-RPC
// arguments avoids needing to hand-escape a file path for Lua/Ex-command
// syntax altogether.
const jumpLua = `local file, line = ...
vim.cmd.edit(vim.fn.fnameescape(file))
vim.api.nvim_win_set_cursor(0, {line, 0})`

func (c *realNvimClient) Jump(file string, line int) error {
	if !filepath.IsAbs(file) {
		return fmt.Errorf("internal error: jump target must be absolute, got %q", file)
	}
	return c.nv.ExecLua(jumpLua, nil, file, line)
}
