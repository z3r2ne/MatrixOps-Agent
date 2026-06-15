package tool

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type GrepTool struct{}

func (GrepTool) Name() string {
	return "grep"
}

func (GrepTool) VerbosName() string {
	return "内容搜索"
}

func (GrepTool) Description() string {
	return "按正则搜索文件内容"
}

func (GrepTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"pattern": map[string]interface{}{
			"type":        "string",
			"description": "The regex pattern to search for",
		},
		"path": map[string]interface{}{
			"type":        "string",
			"description": "The path or directory to search in",
		},
	}, []string{"pattern"})
}

func (GrepTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	pattern, ok := input["pattern"].(string)
	if !ok || pattern == "" {
		return Result{IsError: true}, errors.New("grep: missing pattern")
	}
	root := ctx.Directory
	if path, ok := input["path"].(string); ok && path != "" {
		root = resolvePath(ctx.Directory, path)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return Result{IsError: true}, err
	}
	var matches []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		line := 1
		for scanner.Scan() {
			text := scanner.Text()
			if re.MatchString(text) {
				matches = append(matches, path+":"+itoa(line)+":"+text)
			}
			line++
		}
		return nil
	})
	if err != nil {
		return Result{IsError: true}, err
	}
	sort.Strings(matches)
	return Result{Content: strings.Join(matches, "\n")}, nil
}

func itoa(value int) string {
	return strconv.FormatInt(int64(value), 10)
}
