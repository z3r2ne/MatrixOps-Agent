package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type TreeTool struct{}

func (TreeTool) Name() string {
	return "tree"
}

func (TreeTool) VerbosName() string {
	return "目录树"
}

func (TreeTool) Description() string {
	return "以树形结构显示目录内容，支持指定层级深度（最大10层）；适合局部目录结构确认，不适合作为广泛仓库摸排的第一步；跨目录只读探索优先考虑 `run_worker_task` + `explore`"
}

func (TreeTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "目录路径，默认为当前目录",
		},
		"depth": map[string]interface{}{
			"type":        "number",
			"description": "显示的层级深度，默认为3，最大为10",
		},
	}, nil)
}

func (TreeTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	path, _ := input["path"].(string)
	if path == "" {
		path = "."
	}

	// 获取深度参数
	depth := 3 // 默认深度
	if d, ok := input["depth"].(float64); ok {
		depth = int(d)
	}
	if depth < 1 {
		depth = 1
	}
	if depth > 10 {
		depth = 10
	}

	resolved := resolvePath(ctx.Directory, path)

	// 检查路径是否存在
	info, err := os.Stat(resolved)
	if err != nil {
		return Result{IsError: true}, err
	}
	if !info.IsDir() {
		return Result{IsError: true}, fmt.Errorf("path is not a directory: %s", path)
	}

	var result strings.Builder
	result.WriteString(filepath.Base(resolved))
	result.WriteString("/\n")

	// 统计信息
	stats := &treeStats{
		dirs:  0,
		files: 0,
	}

	if err := buildTree(resolved, "", depth, 1, stats, &result); err != nil {
		return Result{IsError: true}, err
	}

	// 添加统计信息
	result.WriteString(fmt.Sprintf("\n%d directories, %d files\n", stats.dirs, stats.files))

	return Result{Content: result.String()}, nil
}

type treeStats struct {
	dirs  int
	files int
}

// buildTree 递归构建目录树
func buildTree(dirPath string, prefix string, maxDepth int, currentDepth int, stats *treeStats, builder *strings.Builder) error {
	if currentDepth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	// 过滤隐藏文件并排序
	var filteredEntries []os.DirEntry
	for _, entry := range entries {
		// 跳过 .git 和其他隐藏文件
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		filteredEntries = append(filteredEntries, entry)
	}

	// 排序：目录在前，文件在后
	sort.Slice(filteredEntries, func(i, j int) bool {
		if filteredEntries[i].IsDir() != filteredEntries[j].IsDir() {
			return filteredEntries[i].IsDir()
		}
		return filteredEntries[i].Name() < filteredEntries[j].Name()
	})

	for i, entry := range filteredEntries {
		isLast := i == len(filteredEntries)-1
		name := entry.Name()

		// 选择合适的树形符号
		var connector, extender string
		if isLast {
			connector = "└── "
			extender = "    "
		} else {
			connector = "├── "
			extender = "│   "
		}

		// 写入当前项
		builder.WriteString(prefix)
		builder.WriteString(connector)
		if entry.IsDir() {
			builder.WriteString(name)
			builder.WriteString("/\n")
			stats.dirs++

			// 递归处理子目录
			subPath := filepath.Join(dirPath, name)
			newPrefix := prefix + extender
			if err := buildTree(subPath, newPrefix, maxDepth, currentDepth+1, stats, builder); err != nil {
				// 如果是权限错误，跳过该目录
				if os.IsPermission(err) {
					builder.WriteString(newPrefix)
					builder.WriteString("[permission denied]\n")
					continue
				}
				return err
			}
		} else {
			builder.WriteString(name)
			builder.WriteString("\n")
			stats.files++
		}
	}

	return nil
}
