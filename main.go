package main

import "gitte/cmd"

// version is set at build time via -ldflags "-X main.version=<tag>"
var version string

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
