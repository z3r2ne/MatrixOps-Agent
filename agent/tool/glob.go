package tool

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type GlobTool struct{}

func (GlobTool) Name() string {
	return "glob"
}

func (GlobTool) VerbosName() string {
	return "文件匹配"
}

func (GlobTool) Description() string {
	return "按 glob 模式匹配文件路径，支持 ** 递归与 {a,b} 花括号（如 **/*.{ts,tsx}、frontend/**/*.go）"
}

func (GlobTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"pattern": map[string]interface{}{
			"type":        "string",
			"description": "Glob pattern relative to root (supports *, **, ?, [abc], {ts,tsx})",
		},
		"root": map[string]interface{}{
			"type":        "string",
			"description": "The root directory to search in",
		},
	}, []string{"pattern"})
}

func (GlobTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	pattern, ok := input["pattern"].(string)
	if !ok || pattern == "" {
		return Result{IsError: true}, errors.New("glob: missing pattern")
	}
	base := ctx.Directory
	if root, ok := input["root"].(string); ok && root != "" {
		base = resolvePath(ctx.Directory, root)
	}
	base = filepath.Clean(base)

	pattern = filepath.ToSlash(strings.TrimPrefix(filepath.ToSlash(pattern), "/"))
	if pattern == "" {
		return Result{IsError: true}, errors.New("glob: empty pattern")
	}

	relMatches, err := doublestar.Glob(os.DirFS(base), pattern)
	if err != nil {
		return Result{IsError: true}, err
	}

	matches := make([]string, 0, len(relMatches))
	for _, rel := range relMatches {
		full, ok := globMatchFileUnderBase(base, rel)
		if ok {
			matches = append(matches, full)
		}
	}
	sort.Strings(matches)
	return Result{Content: strings.Join(matches, "\n")}, nil
}

func globMatchFileUnderBase(base, rel string) (string, bool) {
	full := filepath.Join(base, filepath.FromSlash(rel))
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return "", false
	}
	cleanBase := filepath.Clean(base)
	cleanFull := filepath.Clean(full)
	if cleanFull != cleanBase && !strings.HasPrefix(cleanFull, cleanBase+string(filepath.Separator)) {
		return "", false
	}
	return cleanFull, true
}
