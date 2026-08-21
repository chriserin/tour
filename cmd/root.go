// Package cmd holds tour's cobra command tree.
package cmd

import (
	"errors"
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"tour/viewer"
)

func tourName(path string) string { return filepath.Base(path) }
func tourDir(path string) string  { return filepath.Dir(path) }

var nvimAddr string

var rootCmd = &cobra.Command{
	Use:           "tour <tour-file>",
	Short:         "Tour: guided, narrated walkthroughs of code changes",
	Args:          cobra.ExactArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runRoot,
}

func init() {
	rootCmd.Flags().StringVar(&nvimAddr, "nvim", "",
		"address of a running Neovim's RPC socket, overriding auto-detection")
}

// runRoot launches the viewer, per design/VIEWER.md's Launching section.
func runRoot(cmd *cobra.Command, args []string) error {
	path := args[0]

	slides, parseErrs, locErrs, err := viewer.Load(path)
	if err != nil {
		return err
	}

	// Structural errors mean there's no reliable slide 1 to show — report
	// them and don't open the viewer, per design/VIEWER.md's Validation
	// section. Location errors don't block opening; they're handed to the
	// viewer below so it can mark the affected slides.
	if len(parseErrs) > 0 {
		out := cmd.OutOrStdout()
		style := lipgloss.NewRenderer(out).NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
		fmt.Fprintf(out, "%d error(s) found\n", len(parseErrs))
		for _, e := range parseErrs {
			fmt.Fprintln(out, style.Render("error: "+e.Error()))
		}
		return ErrReported
	}

	addr, locateErr := viewer.LocateNvim(nvimAddr)
	var nvimClient viewer.NvimClient
	var nvimErr error
	if locateErr != nil {
		nvimErr = locateErr
	} else if client, connErr := viewer.ConnectNvim(addr); connErr != nil {
		nvimErr = connErr
	} else {
		nvimClient = client
	}

	m := viewer.New(tourName(path), tourDir(path), slides, locErrs, nvimClient, nvimErr)
	if _, runErr := tea.NewProgram(m).Run(); runErr != nil {
		return runErr
	}
	return nil
}

// Execute runs the command tree, returning any error from the command that
// ran. Callers decide the process exit code from this.
func Execute() error {
	return rootCmd.Execute()
}

// ErrReported is returned by a command that has already printed its own
// user-facing report of what went wrong (per design/PARSE.md's Output
// section, for example) — it carries no message of its own, since nothing
// downstream needs one. It only signals a non-zero exit code; the caller
// should print nothing further. An error reaching the caller that is *not*
// ErrReported is one no command reported itself, and is the caller's own
// job to print.
var ErrReported = errors.New("reported")
