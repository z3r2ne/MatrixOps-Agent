package config

import (
	"bufio"
	"strconv"
	"strings"
)

func parseYAML(input string) (map[string]interface{}, error) {
	root := map[string]interface{}{}
	type frame struct {
		indent int
		node   map[string]interface{}
	}
	stack := []frame{{indent: -1, node: root}}

	scanner := bufio.NewScanner(strings.NewReader(input))
	var pendingKey string
	var pendingIndent int
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))

		if pendingKey != "" && indent > pendingIndent && strings.HasPrefix(strings.TrimSpace(line), "- ") {
			list := []interface{}{}
			for scanner.Scan() {
				itemLine := scanner.Text()
				itemTrim := strings.TrimSpace(itemLine)
				itemIndent := len(itemLine) - len(strings.TrimLeft(itemLine, " "))
				if itemTrim == "" {
					continue
				}
				if itemIndent <= pendingIndent || !strings.HasPrefix(itemTrim, "- ") {
					line = itemLine
					indent = itemIndent
					trimmed = itemTrim
					break
				}
				list = append(list, parseScalar(strings.TrimSpace(strings.TrimPrefix(itemTrim, "- "))))
			}
			current := stack[len(stack)-1].node
			current[pendingKey] = list
			pendingKey = ""
			if trimmed == "" || strings.HasPrefix(trimmed, "- ") {
				continue
			}
		}

		for len(stack) > 0 && indent <= stack[len(stack)-1].indent {
			stack = stack[:len(stack)-1]
		}
		current := stack[len(stack)-1].node

		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if value == "" {
			peekKey := key
			peekIndent := indent
			nested := map[string]interface{}{}
			current[key] = nested
			stack = append(stack, frame{indent: indent, node: nested})
			pendingKey = peekKey
			pendingIndent = peekIndent
			continue
		}
		if strings.HasPrefix(value, ">") || strings.HasPrefix(value, "|") {
			block := []string{}
			blockIndent := indent
			for scanner.Scan() {
				blockLine := scanner.Text()
				blockTrim := strings.TrimSpace(blockLine)
				lineIndent := len(blockLine) - len(strings.TrimLeft(blockLine, " "))
				if blockTrim == "" {
					block = append(block, "")
					continue
				}
				if lineIndent <= blockIndent {
					line = blockLine
					indent = lineIndent
					trimmed = strings.TrimSpace(line)
					break
				}
				block = append(block, strings.TrimLeft(blockLine, " "))
			}
			current[key] = strings.Join(block, "\n")
			continue
		}
		current[key] = parseScalar(value)
	}
	return root, scanner.Err()
}

func parseScalar(value string) interface{} {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, "\r")
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") && len(value) >= 2 {
		return strings.Trim(value, "\"")
	}
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
		return strings.Trim(value, "'")
	}
	switch strings.ToLower(value) {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	if num, ok := parseNumber(value); ok {
		return num
	}
	return value
}

func parseNumber(value string) (interface{}, bool) {
	for _, r := range value {
		if (r < '0' || r > '9') && r != '.' && r != '-' {
			return nil, false
		}
	}
	if strings.Contains(value, ".") {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f, true
		}
		return nil, false
	}
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i, true
	}
	return nil, false
}
