package git

import "strings"

// FileSnapshot 记录某时刻相对 HEAD 的已修改与未跟踪文件列表。
type FileSnapshot struct {
	Modified  []string
	Untracked []string
}

func FileSnapshotFromRepoState(state *RepoState) *FileSnapshot {
	if state == nil {
		return nil
	}
	return &FileSnapshot{
		Modified:  append([]string(nil), state.ModifiedFiles...),
		Untracked: append([]string(nil), state.UntrackedFiles...),
	}
}

// DiffFileSnapshots 返回子任务执行期间新增/变更的文件路径（相对 before 快照）。
func DiffFileSnapshots(before, after *FileSnapshot) (modified []string, created []string) {
	if after == nil {
		return nil, nil
	}
	if before == nil {
		return normalizePathList(after.Modified), normalizePathList(after.Untracked)
	}

	beforeModified := pathSet(before.Modified)
	beforeUntracked := pathSet(before.Untracked)
	modified = pathsNotInSet(normalizePathList(after.Modified), beforeModified)
	created = pathsNotInSet(normalizePathList(after.Untracked), unionPathSets(beforeUntracked, beforeModified))
	return modified, created
}

func unionPathSets(sets ...map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{})
	for _, set := range sets {
		for path := range set {
			out[path] = struct{}{}
		}
	}
	return out
}

func unionPaths(a, b []string) []string {
	out := make([]string, 0, len(a)+len(b))
	seen := make(map[string]struct{}, len(a)+len(b))
	for _, list := range [][]string{a, b} {
		for _, item := range list {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			out = append(out, trimmed)
		}
	}
	return out
}

func pathSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range normalizePathList(items) {
		set[item] = struct{}{}
	}
	return set
}

func pathsNotInSet(items []string, excluded map[string]struct{}) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := excluded[item]; ok {
			continue
		}
		out = append(out, item)
	}
	return out
}

func normalizePathList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
