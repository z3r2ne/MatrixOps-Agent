package git

import (
	"errors"
	"strings"
)

// RevParse 在 workDir 仓库中将 ref 解析为完整提交或对象哈希。
func RevParse(workDir, ref string) (string, error) {
	if workDir == "" || strings.TrimSpace(ref) == "" {
		return "", errors.New("workdir and ref required")
	}
	out, err := run(workDir, "rev-parse", strings.TrimSpace(ref))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
