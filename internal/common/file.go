package common

import (
	"fmt"
	"io"
	"os"
)

func ReadFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return nil, fmt.Errorf("😡 Could not open %s: %v", path, err)
	}

	body, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("😡 Could not read %s: %v", path, err)
	}

	return body, nil
}
