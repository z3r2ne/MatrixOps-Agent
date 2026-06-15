package tool

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ListTool struct{}

func (ListTool) Name() string {
	return "list"
}

func (ListTool) VerbosName() string {
	return "目录列表"
}

func (ListTool) Description() string {
	return "列出目录下的文件；适合局部目录确认，不适合作为广泛仓库摸排的第一步；跨目录只读探索优先考虑 `run_worker_task` + `explore`"
}

func (ListTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "The directory path to list (use `paths` to list multiple directories)",
		},
		"paths": map[string]interface{}{
			"type":        "array",
			"description": "Multiple directory paths to list",
			"items": map[string]interface{}{
				"type": "string",
			},
		},
	}, nil)
}

func (ListTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	paths := listPathsFromInput(input)
	if len(paths) == 0 {
		return Result{IsError: true}, errors.New("list: missing path")
	}

	if len(paths) == 1 {
		content, err := listDirectory(ctx.Directory, paths[0])
		if err != nil {
			return Result{IsError: true}, err
		}
		return Result{Content: content}, nil
	}

	var builder strings.Builder
	errs := make([]string, 0)
	for index, path := range paths {
		if index > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString("# ")
		builder.WriteString(path)
		builder.WriteString("\n")

		content, err := listDirectory(ctx.Directory, path)
		if err != nil {
			builder.WriteString("[Error: ")
			builder.WriteString(err.Error())
			builder.WriteString("]")
			errs = append(errs, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		if content == "" {
			builder.WriteString("(empty)")
		} else {
			builder.WriteString(content)
		}
	}

	if len(errs) == len(paths) {
		return Result{IsError: true}, fmt.Errorf("list: %s", strings.Join(errs, "; "))
	}
	return Result{Content: builder.String()}, nil
}

func listPathsFromInput(input map[string]interface{}) []string {
	if paths := stringSliceFrom(input["paths"]); len(paths) > 0 {
		filtered := make([]string, 0, len(paths))
		for _, path := range paths {
			path = strings.TrimSpace(path)
			if path != "" {
				filtered = append(filtered, path)
			}
		}
		if len(filtered) > 0 {
			return filtered
		}
	}

	if path, ok := input["path"].(string); ok && strings.TrimSpace(path) != "" {
		return []string{strings.TrimSpace(path)}
	}
	return []string{"."}
}

func listDirectory(baseDir, path string) (string, error) {
	resolved := resolvePath(baseDir, path)
	entries, err := os.ReadDir(resolved)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += string(filepath.Separator)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, "\n"), nil
}

func requirePath(input map[string]interface{}) (string, error) {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return "", errors.New("missing path")
	}
	return path, nil
}
