package startup

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/term"
)

func TestTUISummary_ZeroWidthTerminal(t *testing.T) {
	const diagnostic = "This diagnostic should fit on a single line in an eighty column terminal."
	if os.Getenv("GITTE_TEST_ZERO_WIDTH_TERMINAL") == "1" {
		width, _, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil || width != 0 {
			t.Fatalf("terminal width = %d, error = %v; want zero width without error", width, err)
		}
		v := &tuiView{summary: newCheckSummary()}
		v.summary.record("example", errors.New(diagnostic), 0)
		v.printFailureSummary()
		return
	}
	if runtime.GOOS != "linux" {
		t.Skip("uses the Linux script command to create a zero-width terminal")
	}
	script, err := exec.LookPath("script")
	if err != nil {
		t.Skip("script is unavailable")
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := "stty cols 0 && exec '" + strings.ReplaceAll(binary, "'", "'\\''") + "' -test.run=^TestTUISummary_ZeroWidthTerminal$"
	cmd := exec.CommandContext(ctx, script, "-qec", command, "/dev/null")
	cmd.Env = append(os.Environ(), "GITTE_TEST_ZERO_WIDTH_TERMINAL=1", "NO_COLOR=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("zero-width terminal test: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), diagnostic) {
		t.Fatalf("diagnostic unexpectedly wrapped:\n%s", output)
	}
}
