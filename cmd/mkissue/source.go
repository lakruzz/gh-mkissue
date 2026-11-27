package mkissue

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

type sourceSnapshot struct {
	path string
	info os.FileInfo
	hash [sha256.Size]byte
}

func readLocalSource(path string) ([]byte, sourceSnapshot, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, sourceSnapshot{}, err
	}
	if !info.Mode().IsRegular() {
		return nil, sourceSnapshot{}, fmt.Errorf("source must be a regular file, not a symlink")
	}
	// #nosec G304 -- source path is explicitly selected by the user
	file, err := os.Open(path)
	if err != nil {
		return nil, sourceSnapshot{}, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, sourceSnapshot{}, fmt.Errorf("source changed while opening")
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, sourceSnapshot{}, err
	}
	return content, sourceSnapshot{path: path, info: info, hash: sha256.Sum256(content)}, nil
}

func cleanSource(snapshot sourceSnapshot) error {
	info, err := os.Lstat(snapshot.path)
	if err != nil {
		return fmt.Errorf("inspect source: %w", err)
	}
	if !info.Mode().IsRegular() || !os.SameFile(snapshot.info, info) {
		return fmt.Errorf("source was replaced; leaving it in place")
	}
	// #nosec G304 -- source path was selected by the user and validated above
	content, err := os.ReadFile(snapshot.path)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}
	if sha256.Sum256(content) != snapshot.hash {
		return fmt.Errorf("source was modified; leaving it in place")
	}
	if err := os.Remove(snapshot.path); err != nil {
		return fmt.Errorf("remove source: %w", err)
	}
	return nil
}
