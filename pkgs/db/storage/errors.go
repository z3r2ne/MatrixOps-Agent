package storage

import "fmt"

type NotFoundError struct {
	Path string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("resource not found: %s", e.Path)
}
