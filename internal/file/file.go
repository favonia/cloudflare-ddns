package file

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

func ReadFileAsString(path string) (string, bool) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("😡 Could not open %s: %v\n", path, err)
		return "", false
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		fmt.Printf("😡 Could not read %s: %v\n", path, err)
		return "", false
	}

	return string(bytes.TrimSpace(content)), true
}
