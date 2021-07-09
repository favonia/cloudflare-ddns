package file

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

func ReadFileAsString(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("😡 Could not open %s: %v", path, err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("😡 Could not read %s: %v", path, err)
	}

	return string(bytes.TrimSpace(content)), nil
}
