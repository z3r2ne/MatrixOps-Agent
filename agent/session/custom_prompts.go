package session

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"matrixops-agent/config"
	"matrixops-agent/global"
	"matrixops-agent/taskctx"
	"pkgs/db/models"
	"pkgs/skillfs"
)

// CustomWithSource 收集所有自定义 prompt 的来源和内容。
// 返回 [][]string，每个元素为 [source, content]。
func CustomWithSource(task *models.Task) ([][]string, error) {
	if err := global.Init(); err != nil {
		return nil, err
	}
	cfg, err := config.Get(task)
	if err != nil {
		return nil, err
	}
	ctx, err := taskctx.Resolve(task)
	if err != nil {
		return nil, err
	}

	paths := map[string]struct{}{}
	if !envBool(global.EnvDisableProjectConfig) {
		for _, localRuleFile := range []string{"AGENTS.md", "CLAUDE.md", "CONTEXT.md"} {
			matches := findUp(localRuleFile, ctx.WorkDir, ctx.Worktree)
			if len(matches) > 0 {
				for _, match := range matches {
					paths[match] = struct{}{}
				}
				break
			}
		}
	}

	globalRuleFiles := []string{filepath.Join(global.Path.Config, "AGENTS.md")}
	if !envBool(global.EnvDisableClaudeCodePrompt) {
		if home, err := os.UserHomeDir(); err == nil {
			globalRuleFiles = append(globalRuleFiles, filepath.Join(home, ".claude", "CLAUDE.md"))
		}
	}
	if extra := os.Getenv(global.EnvConfigDir); extra != "" {
		globalRuleFiles = append(globalRuleFiles, filepath.Join(extra, "AGENTS.md"))
	}
	for _, globalRuleFile := range globalRuleFiles {
		if fileExists(globalRuleFile) {
			paths[globalRuleFile] = struct{}{}
			break
		}
	}
	urls := []string{}
	for _, instruction := range cfg.Instructions {
		if strings.HasPrefix(instruction, "https://") || strings.HasPrefix(instruction, "http://") {
			urls = append(urls, instruction)
			continue
		}
		if strings.HasPrefix(instruction, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				instruction = filepath.Join(home, instruction[2:])
			}
		}
		var matches []string
		if filepath.IsAbs(instruction) {
			matches = globAbsolute(instruction)
		} else {
			matches = resolveRelativeInstruction(instruction, ctx)
		}
		for _, match := range matches {
			paths[match] = struct{}{}
		}
	}

	filePaths := make([]string, 0, len(paths))
	for path := range paths {
		filePaths = append(filePaths, path)
	}
	sort.Strings(filePaths)

	results := [][]string{}
	for _, path := range filePaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := strings.TrimRight(string(data), "\n\r")
		if strings.TrimSpace(text) == "" {
			continue
		}
		results = append(results, []string{path, text})
	}

	client := http.Client{Timeout: 5 * time.Second}
	for _, url := range urls {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			continue
		}
		text := strings.TrimRight(string(body), "\n\r")
		if strings.TrimSpace(text) == "" {
			continue
		}
		results = append(results, []string{url, text})
	}
	return results, nil
}

func resolveRelativeInstruction(instruction string, ctx taskctx.Context) []string {
	if ctx.WorkDir == "" {
		return []string{}
	}
	if !envBool(global.EnvDisableProjectConfig) {
		return globUp(instruction, ctx.WorkDir, ctx.Worktree)
	}
	if configDir := os.Getenv(global.EnvConfigDir); configDir != "" {
		return globUp(instruction, configDir, configDir)
	}
	return []string{}
}

func globAbsolute(pattern string) []string {
	if !hasMeta(pattern) {
		if fileExists(pattern) {
			return []string{pattern}
		}
		return []string{}
	}
	base := basePathForPattern(pattern)
	rel := strings.TrimPrefix(filepath.Clean(pattern), base)
	rel = strings.TrimPrefix(rel, string(filepath.Separator))
	return globPattern(base, rel)
}

func globUp(pattern string, start string, stop string) []string {
	var results []string
	current := start
	for {
		results = append(results, globPattern(current, pattern)...)
		if current == stop {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return uniqueStrings(results)
}

func findUp(target string, start string, stop string) []string {
	var results []string
	current := start
	for {
		candidate := filepath.Join(current, target)
		if fileExists(candidate) {
			results = append(results, candidate)
		}
		if current == stop {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return results
}

func globPattern(root string, pattern string) []string {
	if !hasMeta(pattern) {
		path := filepath.Join(root, pattern)
		if fileExists(path) {
			return []string{path}
		}
		return []string{}
	}
	if !strings.Contains(pattern, "**") {
		matches, _ := filepath.Glob(filepath.Join(root, pattern))
		return filterFiles(matches)
	}
	return walkMatch(root, pattern)
}

func installedSkillSummaryInstruction() string {
	skills, err := skillfs.ListInstalledSkills()
	if err != nil || len(skills) == 0 {
		return ""
	}
	lines := []string{
		"Installed skills (call `load_skill` with the exact name when needed):",
		"Full skill content is returned in the tool output and kept in conversation history; it is not injected into the static system prompt.",
	}
	for _, skill := range skills {
		line := "- " + strings.TrimSpace(skill.Name)
		if desc := strings.TrimSpace(skill.Description); desc != "" {
			line += ": " + desc
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func walkMatch(root string, pattern string) []string {
	var matches []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if matchPattern(pattern, rel) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches
}

func matchPattern(pattern string, name string) bool {
	pattern = filepath.ToSlash(pattern)
	name = filepath.ToSlash(name)
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); {
		ch := pattern[i]
		if ch == '*' {
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i += 2
				continue
			}
			b.WriteString(`[^/]*`)
			i++
			continue
		}
		if ch == '?' {
			b.WriteString(".")
			i++
			continue
		}
		b.WriteString(regexpQuote(string(ch)))
		i++
	}
	b.WriteString("$")
	matched, _ := regexpMatch(b.String(), name)
	return matched
}

func regexpQuote(value string) string {
	switch value {
	case ".", "+", "(", ")", "|", "^", "$", "{", "}", "[", "]", "\\", "/":
		return `\` + value
	default:
		return value
	}
}

func regexpMatch(pattern string, value string) (bool, error) {
	return regexp.MatchString(pattern, value)
}

func basePathForPattern(pattern string) string {
	clean := filepath.Clean(pattern)
	slash := filepath.ToSlash(clean)
	parts := strings.Split(slash, "/")
	var baseParts []string
	for _, part := range parts {
		if hasMeta(part) {
			break
		}
		baseParts = append(baseParts, part)
	}
	if len(baseParts) == 0 {
		return string(filepath.Separator)
	}
	base := filepath.Join(baseParts...)
	if strings.HasPrefix(clean, string(filepath.Separator)) {
		base = string(filepath.Separator) + base
	}
	return filepath.Clean(base)
}

func hasMeta(value string) bool {
	return strings.ContainsAny(value, "*?[")
}

func filterFiles(paths []string) []string {
	out := []string{}
	for _, path := range paths {
		if fileExists(path) {
			out = append(out, path)
		}
	}
	return out
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func uniqueStrings(input []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, item := range input {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func envBool(key string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return value == "1" || value == "true" || value == "yes"
}

