package output

import (
	"os"

	"golang.org/x/term"
)

// OutputMode controls how output is rendered
type OutputMode int

const (
	ModeTTY   OutputMode = iota // BubbleTea TUI
	ModePlain                   // Plain structured text
)

// IsTTY returns true if stdout is a terminal and GITTE_NO_TTY is not set
func IsTTY() bool {
	if os.Getenv("GITTE_NO_TTY") == "1" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// DetectMode returns the appropriate output mode
func DetectMode(noTTY bool) OutputMode {
	if noTTY || !IsTTY() {
		return ModePlain
	}
	return ModeTTY
}
