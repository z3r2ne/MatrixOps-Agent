package database

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"pkgs/db/models"

	"gorm.io/gorm"
)

const (
	builtInSkillSourceName    = "MatrixOps 内置技能"
	builtInSkillSourceRepoURL = "builtin://matrixops"
)

// IsBuiltInSkillSource 判断是否为内置技能源地址。
func IsBuiltInSkillSource(repoURL string) bool {
	return strings.TrimSpace(repoURL) == builtInSkillSourceRepoURL
}

// EnsureBuiltInSkillSource 确保内置技能源存在于数据库中。
func EnsureBuiltInSkillSource(db *gorm.DB) (*models.SkillSource, error) {
	var source models.SkillSource
	err := db.Where("repo_url = ?", builtInSkillSourceRepoURL).First(&source).Error
	if err == nil {
		return &source, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	source = models.SkillSource{
		Name:       builtInSkillSourceName,
		RepoURL:    builtInSkillSourceRepoURL,
		SkillsPath: ".",
		Enabled:    true,
	}
	if err := CreateSkillSource(db, &source); err != nil {
		return nil, err
	}
	return &source, nil
}

// InitBuiltInSkills 将缺失的内置技能安装到本地；已安装的技能保持不变。
func InitBuiltInSkills(db *gorm.DB, files map[string][]byte) error {
	return applyBuiltInSkills(db, files, false)
}

// RestoreBuiltInSkills 强制覆盖安装所有内置技能，用于恢复默认版本。
func RestoreBuiltInSkills(db *gorm.DB, files map[string][]byte) error {
	return applyBuiltInSkills(db, files, true)
}

func applyBuiltInSkills(db *gorm.DB, files map[string][]byte, overwriteExisting bool) error {
	if len(files) == 0 {
		return nil
	}
	source, err := EnsureBuiltInSkillSource(db)
	if err != nil {
		return err
	}
	grouped := groupBuiltInSkillFiles(files)
	relativePaths := make([]string, 0, len(grouped))
	for relativePath := range grouped {
		relativePaths = append(relativePaths, relativePath)
	}
	sort.Strings(relativePaths)

	sourceRoot, err := SkillSourceLocalPath(source.ID)
	if err != nil {
		return err
	}

	for _, relativePath := range relativePaths {
		skillFiles := grouped[relativePath]
		cacheDir := filepath.Join(sourceRoot, relativePath)
		installedPath, err := InstalledSkillPath(source.ID, relativePath)
		if err != nil {
			return err
		}

		if overwriteExisting {
			_ = os.RemoveAll(cacheDir)
			_ = os.RemoveAll(installedPath)
		}

		if err := writeBuiltInSkillDir(cacheDir, skillFiles); err != nil {
			return err
		}
		if overwriteExisting || !isBuiltInSkillInstalled(installedPath) {
			if err := writeBuiltInSkillDir(installedPath, skillFiles); err != nil {
				return err
			}
		}
	}

	now := time.Now()
	return db.Model(source).Updates(map[string]interface{}{
		"local_path":      sourceRoot,
		"skill_count":     len(grouped),
		"last_sync_at":    &now,
		"last_sync_error": "",
	}).Error
}

func groupBuiltInSkillFiles(files map[string][]byte) map[string]map[string][]byte {
	grouped := make(map[string]map[string][]byte)
	for key, content := range files {
		key = filepath.ToSlash(strings.TrimSpace(key))
		if key == "" || len(content) == 0 {
			continue
		}
		dir := filepath.Dir(key)
		if dir == "." || dir == "" {
			continue
		}
		fileName := filepath.Base(key)
		if grouped[dir] == nil {
			grouped[dir] = make(map[string][]byte)
		}
		grouped[dir][fileName] = content
	}
	return grouped
}

func writeBuiltInSkillDir(dir string, files map[string][]byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		target := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, files[name], 0o644); err != nil {
			return err
		}
	}
	return nil
}

func isBuiltInSkillInstalled(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "SKILL.md"))
	return err == nil && !info.IsDir()
}
