package main

import (
	"errors"
	"fmt"
	"os"

	"tour/cmd"
)

func main() {
	err := cmd.Execute()
	if err == nil {
		return
	}
	if !errors.Is(err, cmd.ErrReported) {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	os.Exit(1)
}
