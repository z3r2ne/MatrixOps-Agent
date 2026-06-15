package session

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	tempUploadDirOnce sync.Once
	tempUploadDir     string
)

// TempUploadDir 返回全局临时上传目录（os.TempDir()/matrixops-temp-uploads）。
func TempUploadDir() string {
	tempUploadDirOnce.Do(func() {
		tempUploadDir = filepath.Join(os.TempDir(), "matrixops-temp-uploads")
		_ = os.MkdirAll(tempUploadDir, 0o755)
	})
	return tempUploadDir
}

// SaveTempUserInputFile 将用户上传的文件写入全局临时目录，返回绝对路径。
func SaveTempUserInputFile(filename string, data []byte) (absPath string, err error) {
	name := sanitizeUserInputFilename(filename)
	if name == "" {
		name = "attachment.bin"
	}
	name = fmt.Sprintf("%d_%s", time.Now().UnixMilli(), name)
	dir := filepath.Join(TempUploadDir(), UserInputSubdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create temp user input dir: %w", err)
	}
	absPath = filepath.Join(dir, name)
	if err := os.WriteFile(absPath, data, 0o600); err != nil {
		return "", fmt.Errorf("write temp user input file: %w", err)
	}
	return filepath.Clean(absPath), nil
}

// ResolveTempUserInputPath 解析相对于 TempUploadDir 的用户输入文件路径。
func ResolveTempUserInputPath(relativePath string) (string, error) {
	return ResolveUserInputPath(TempUploadDir(), relativePath)
}

// ResolveStoredUserInputFilePath 解析用户附件 path（绝对路径、临时目录相对路径，或旧版 workDir 相对路径）。
func ResolveStoredUserInputFilePath(path string, workDir string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(path) {
		return validateUnderTempUploadDir(filepath.Clean(path))
	}
	if abs, err := ResolveTempUserInputPath(path); err == nil {
		return abs, nil
	}
	workDir = strings.TrimSpace(workDir)
	if workDir != "" {
		return ResolveUserInputPath(workDir, path)
	}
	return "", fmt.Errorf("invalid user input file path %q", path)
}

func validateUnderTempUploadDir(absPath string) (string, error) {
	tempRoot := filepath.Clean(TempUploadDir())
	if absPath != tempRoot && !strings.HasPrefix(absPath, tempRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path must be under temp upload dir")
	}
	userRoot := filepath.Join(tempRoot, UserInputSubdir)
	if absPath != userRoot && !strings.HasPrefix(absPath, userRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path must be under %s", UserInputSubdir)
	}
	return absPath, nil
}

// TempUserInputFileAPIURL 返回临时文件预览 URL（path 为绝对路径或临时目录相对路径）。
func TempUserInputFileAPIURL(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return fmt.Sprintf("/api/temp-uploads?path=%s", url.QueryEscape(path))
}
