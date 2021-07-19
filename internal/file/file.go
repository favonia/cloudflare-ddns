package file

import (
	"bytes"
	"io"
	"log"
	"os"
)

func ReadFileAsString(path string) (string, bool) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("😡 Could not open %s: %v", path, err)
		return "", false //nolint:nlreturn
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		log.Printf("😡 Could not read %s: %v", path, err)
		return "", false //nolint:nlreturn
	}

	return string(bytes.TrimSpace(content)), true
}
