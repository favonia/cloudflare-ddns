package main

import (
	"context"
	"os/exec"
	"strings"
)

var root = mustProjectRoot()

func mustProjectRoot() string {
	out, err := exec.CommandContext(context.Background(), "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		panic("cannot determine repo root: " + err.Error())
	}
	return strings.TrimSpace(string(out))
}
