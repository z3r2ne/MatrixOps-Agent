package git

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// LogCommit 单条提交（用于时间线）。
type LogCommit struct {
	Hash    string `json:"hash"`
	UnixSec int64  `json:"unixSec"`
	Subject string `json:"subject"`
}

// LogCommitsSince 列出 since..HEAD 上的提交（不含 since本身），按时间从新到旧。
// since 为空时等价于 git log HEAD（仍受 max 限制）。
func LogCommitsSince(workDir, since string, max int) ([]LogCommit, error) {
	if workDir == "" {
		return nil, errors.New("workdir required")
	}
	if max <= 0 {
		max = 100
	}
	if max > 500 {
		max = 500
	}
	var revRange string
	if strings.TrimSpace(since) == "" {
		revRange = "HEAD"
	} else {
		revRange = fmt.Sprintf("%s..HEAD", strings.TrimSpace(since))
	}
	out, err := run(workDir, "log", revRange, fmt.Sprintf("-n%d", max), "--format=%H|%ct|%s")
	if err != nil {
		return nil, err
	}
	var list []LogCommit
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		sec, _ := strconv.ParseInt(parts[1], 10, 64)
		list = append(list, LogCommit{
			Hash:    parts[0],
			UnixSec: sec,
			Subject: parts[2],
		})
	}
	return list, nil
}
