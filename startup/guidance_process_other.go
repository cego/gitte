//go:build !unix

package startup

import "os/exec"

func prepareGuidanceProcess(_ *exec.Cmd) func() { return func() {} }
