package common

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

func ReadFileAsString(path string) (string, error) {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return "", fmt.Errorf("😡 Could not open %s: %v", path, err)
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("😡 Could not read %s: %v", path, err)
	}

	return string(bytes.TrimSpace(content)), nil
}
