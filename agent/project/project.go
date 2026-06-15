package project

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
)

type Info struct {
	ID  string
	VCS string
}

func FromDirectory(directory string) (Info, string, error) {
	gitRoot := findGitRoot(directory)
	if gitRoot == "" {
		return Info{
			ID:  hashID(directory),
			VCS: "none",
		}, string(filepath.Separator), nil
	}
	return Info{
		ID:  hashID(gitRoot),
		VCS: "git",
	}, gitRoot, nil
}

func findGitRoot(start string) string {
	current := start
	for {
		if exists(filepath.Join(current, ".git")) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func hashID(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}
