package project

import (
	"path/filepath"
	"strings"
	"sync"
)

type Instance struct {
	Directory string
	Worktree  string
	Project   Info
}

var (
	cacheMu sync.Mutex
	cache   = map[string]*Instance{}
	currMu  sync.Mutex
	current *Instance
)

func Provide(directory string, init func(*Instance) error, fn func() error) error {
	inst, created, err := getOrCreate(directory)
	if err != nil {
		return err
	}
	if created && init != nil {
		if err := init(inst); err != nil {
			return err
		}
	}
	currMu.Lock()
	prev := current
	current = inst
	currMu.Unlock()
	defer func() {
		currMu.Lock()
		current = prev
		currMu.Unlock()
	}()
	return fn()
}

func Current() *Instance {
	currMu.Lock()
	defer currMu.Unlock()
	return current
}

func Dispose(directory string) {
	cacheMu.Lock()
	delete(cache, directory)
	cacheMu.Unlock()
}

func ContainsPath(inst *Instance, path string) bool {
	if inst == nil {
		return false
	}
	if contains(inst.Directory, path) {
		return true
	}
	if inst.Worktree == string(filepath.Separator) {
		return false
	}
	return contains(inst.Worktree, path)
}

func getOrCreate(directory string) (*Instance, bool, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if inst, ok := cache[directory]; ok {
		return inst, false, nil
	}
	info, worktree, err := FromDirectory(directory)
	if err != nil {
		return nil, false, err
	}
	inst := &Instance{
		Directory: directory,
		Worktree:  worktree,
		Project:   info,
	}
	cache[directory] = inst
	return inst, true, nil
}

func contains(root string, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
