package tool

import (
	"fmt"
	"strconv"
	"strings"
)

// maxReadLines and maxReadBytes cap each read call. When limit is omitted, the tool
// prefers reading the whole file from offset up to these limits (whichever is hit first).
const maxReadLines = 1000
const maxReadBytes = 100 * 1024

// FileReadInfo describes a sliced read of file text.
type FileReadInfo struct {
	Content           string
	TotalLines        int
	ReturnedLines     int
	StartLine         int // 1-based line number of the first returned line
	OffsetUsed        int // 0-based offset actually applied
	LimitUsed         int // effective line cap (0 means through end of slice)
	HasMore           bool
	BeyondEnd         bool
	MaxLinesReached   bool
	MaxBytesReached   bool
}

// sliceFileLines returns a view of file lines and the 1-based line number of the first line.
// offset and limit follow the read tool contract: offset is a 0-based line index; limit is a max
// line count (0 means read up to maxReadLines from offset, or through EOF if shorter).
func sliceFileLines(text string, offset, limit int) (lines []string, startLine int, limitUsed int, hasMore bool, beyondEnd bool) {
	all := strings.Split(text, "\n")
	total := len(all)

	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return nil, 0, 0, false, true
	}

	effectiveLimit := limit
	if effectiveLimit <= 0 {
		remaining := total - offset
		if remaining > maxReadLines {
			effectiveLimit = maxReadLines
		} else {
			effectiveLimit = remaining
		}
	} else if effectiveLimit > maxReadLines {
		effectiveLimit = maxReadLines
	}

	end := total
	if effectiveLimit > 0 && offset+effectiveLimit < end {
		end = offset + effectiveLimit
	}
	hasMore = end < total
	return all[offset:end], offset + 1, effectiveLimit, hasMore, false
}

func renderedLineBytes(renderedLine string, isFirst bool) int {
	n := len([]byte(renderedLine))
	if !isFirst {
		n++
	}
	return n
}

func formatLinesWithLineNumbersLimited(lines []string, startLine int) (formatted string, returned int, maxBytesReached bool) {
	if len(lines) == 0 {
		return "", 0, false
	}

	var b strings.Builder
	bytes := 0
	for i, line := range lines {
		rendered := strconv.Itoa(startLine+i) + "\t" + line
		lineBytes := renderedLineBytes(rendered, i == 0)
		if i > 0 && bytes+lineBytes > maxReadBytes {
			maxBytesReached = true
			break
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(rendered)
		bytes += lineBytes
		returned++
		if bytes >= maxReadBytes {
			maxBytesReached = true
			break
		}
	}
	return b.String(), returned, maxBytesReached
}

func readFileContent(text string, offset, requestedLimit int) FileReadInfo {
	lines, startLine, limitUsed, hasMoreFromSlice, beyondEnd := sliceFileLines(text, offset, requestedLimit)
	total := len(strings.Split(text, "\n"))

	if beyondEnd {
		return FileReadInfo{
			TotalLines: total,
			OffsetUsed: offset,
			BeyondEnd:  true,
		}
	}

	formatted, returned, maxBytesReached := formatLinesWithLineNumbersLimited(lines, startLine)
	maxLinesReached := hasMoreFromSlice && returned >= len(lines)
	hasMore := hasMoreFromSlice || returned < len(lines)

	return FileReadInfo{
		Content:         formatted,
		TotalLines:      total,
		ReturnedLines:   returned,
		StartLine:       startLine,
		OffsetUsed:      offset,
		LimitUsed:       limitUsed,
		HasMore:         hasMore,
		MaxLinesReached: maxLinesReached,
		MaxBytesReached: maxBytesReached,
	}
}

// formatLinesWithLineNumbers prefixes each line as "{n}\t{content}" (1-based n), matching Kimi CLI read output.
func formatLinesWithLineNumbers(lines []string, startLine int) string {
	formatted, _, _ := formatLinesWithLineNumbersLimited(lines, startLine)
	return formatted
}

func formatFileTextWithLineNumbers(text string, offset, limit int) (string, error) {
	return readFileContent(text, offset, limit).Content, nil
}

// BuildReadToolSystemMessageFromMetadata 根据 read 工具 metadata 重建 system 摘要（用于 DB 重载后补全）。
func BuildReadToolSystemMessageFromMetadata(meta map[string]interface{}) string {
	if len(meta) == 0 {
		return ""
	}
	info := FileReadInfo{
		TotalLines:      metadataInt(meta, "total_lines"),
		ReturnedLines:   metadataInt(meta, "lines_returned"),
		StartLine:       metadataInt(meta, "start_line"),
		OffsetUsed:      metadataInt(meta, "offset_used"),
		HasMore:         metadataBool(meta, "has_more"),
		BeyondEnd:       metadataBool(meta, "beyond_end"),
		MaxLinesReached: metadataBool(meta, "max_lines_reached"),
		MaxBytesReached: metadataBool(meta, "max_bytes_reached"),
	}
	if info.ReturnedLines <= 0 && info.TotalLines <= 0 && !info.BeyondEnd {
		return ""
	}
	return BuildReadToolSystemMessage(info)
}

func metadataInt(meta map[string]interface{}, key string) int {
	switch v := meta[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(v))
		return parsed
	default:
		return 0
	}
}

func metadataBool(meta map[string]interface{}, key string) bool {
	switch v := meta[key].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

// BuildReadToolSystemMessage 生成 read 工具的 system 摘要（对齐 Kimi CLI）。
func BuildReadToolSystemMessage(info FileReadInfo) string {
	if info.BeyondEnd {
		return fmt.Sprintf(
			"No lines read from file. Total lines in file: %d. Offset %d is beyond end of file.",
			info.TotalLines,
			info.OffsetUsed,
		)
	}

	startLine := info.StartLine
	if startLine <= 0 && info.ReturnedLines > 0 {
		startLine = info.OffsetUsed + 1
	}

	var message string
	if info.ReturnedLines > 0 {
		message = fmt.Sprintf("%d lines read from file starting from line %d.", info.ReturnedLines, startLine)
	} else {
		message = "No lines read from file."
	}
	message += fmt.Sprintf(" Total lines in file: %d.", info.TotalLines)
	if info.MaxLinesReached {
		message += fmt.Sprintf(" Max %d lines reached.", maxReadLines)
	} else if info.MaxBytesReached {
		message += fmt.Sprintf(" Max %d bytes reached.", maxReadBytes)
	} else if !info.HasMore {
		message += " End of file reached."
	}
	return message
}

func appendReadPaginationMetadata(metadata map[string]interface{}, info FileReadInfo) {
	if metadata == nil {
		return
	}
	metadata["total_lines"] = info.TotalLines
	metadata["lines_returned"] = info.ReturnedLines
	if info.StartLine > 0 {
		metadata["start_line"] = info.StartLine
	}
	metadata["offset_used"] = info.OffsetUsed
	if info.LimitUsed > 0 {
		metadata["limit_used"] = info.LimitUsed
	}
	metadata["has_more"] = info.HasMore
	if info.HasMore {
		metadata["next_offset"] = info.OffsetUsed + info.ReturnedLines
	}
	if info.BeyondEnd {
		metadata["beyond_end"] = true
	}
	if info.MaxLinesReached {
		metadata["max_lines_reached"] = true
	}
	if info.MaxBytesReached {
		metadata["max_bytes_reached"] = true
	}
}
