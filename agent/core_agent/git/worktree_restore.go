package git

import (
	"errors"
	"strings"
)

// RestoreWorktreeTreeish 将工作区内容恢复为给定 tree-ish（提交或树），可选清理未跟踪文件。
func RestoreWorktreeTreeish(workDir, treeish string, clean bool) error {
	if workDir == "" || strings.TrimSpace(treeish) == "" {
		return errors.New("workdir and treeish required")
	}
	ref := strings.TrimSpace(treeish)
	if _, err := run(workDir, "read-tree", ref); err != nil {
		return err
	}
	if _, err := run(workDir, "checkout-index", "-a", "-f"); err != nil {
		return err
	}
	if clean {
		if _, err := run(workDir, "clean", "-fd"); err != nil {
			return err
		}
	}
	return nil
}
