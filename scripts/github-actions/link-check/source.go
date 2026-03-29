package main

import (
	"path/filepath"
	"runtime"
)

var root = mustProjectRoot()

func mustProjectRoot() string {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine source path")
	}
	return filepath.Clean(filepath.Dir(sourceFile))
}
