package memorysearch

import "strings"

const (
	MemorySearchToolName         = "memory_search"
	compressedMemorySourceKind   = "compressed_memory"
	memoryCompactionSearchHint   = "\n\n【提示】刚刚已完成记忆压缩，上文仅为摘要。如需查看被压缩前的完整记忆内容，请使用 memory_search 工具检索。"
)

// AppendMemoryCompactionSearchHint appends guidance for the memory_search tool to compressed summaries.
func AppendMemoryCompactionSearchHint(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if strings.Contains(content, "memory_search 工具") {
		return content
	}
	return content + memoryCompactionSearchHint
}

// StripMemoryCompactionSearchHint removes compaction guidance before indexing searchable text.
func StripMemoryCompactionSearchHint(content string) string {
	return strings.TrimSpace(strings.Replace(content, memoryCompactionSearchHint, "", 1))
}
