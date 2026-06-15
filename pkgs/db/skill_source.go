package database

import (
	"os"
	"path/filepath"
	"strings"

	"pkgs/db/models"

	"gorm.io/gorm"
)

var defaultSkillSources = []models.SkillSource{
	{
		Name:       "Agents Skills",
		RepoURL:    "~/.agents/skills",
		SkillsPath: ".",
		Enabled:    false,
	},
	{
		Name:       "Codex Skills",
		RepoURL:    "~/.codex/skills",
		SkillsPath: ".",
		Enabled:    false,
	},
}

func ListSkillSources(db *gorm.DB) ([]models.SkillSource, error) {
	var sources []models.SkillSource
	err := db.Order("created_at DESC").Find(&sources).Error
	return sources, err
}

func GetSkillSourceByID(db *gorm.DB, id uint) (*models.SkillSource, error) {
	var source models.SkillSource
	if err := db.First(&source, id).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

func CreateSkillSource(db *gorm.DB, source *models.SkillSource) error {
	if source == nil {
		return nil
	}
	if strings.TrimSpace(source.SkillsPath) == "" {
		source.SkillsPath = "skills"
	}
	return db.Create(source).Error
}

func UpdateSkillSource(db *gorm.DB, source *models.SkillSource) error {
	if source == nil {
		return nil
	}
	if strings.TrimSpace(source.SkillsPath) == "" {
		source.SkillsPath = "skills"
	}
	return db.Save(source).Error
}

func DeleteSkillSource(db *gorm.DB, id uint) error {
	return db.Delete(&models.SkillSource{}, id).Error
}

func EnsureDefaultSkillSources(db *gorm.DB) error {
	sources, err := ListSkillSources(db)
	if err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		existing[normalizeSkillSourceAddress(source.RepoURL)] = struct{}{}
	}
	for _, item := range defaultSkillSources {
		key := normalizeSkillSourceAddress(item.RepoURL)
		if _, ok := existing[key]; ok {
			continue
		}
		source := item
		if err := CreateSkillSource(db, &source); err != nil {
			return err
		}
		if err := db.Model(&source).Update("enabled", item.Enabled).Error; err != nil {
			return err
		}
		existing[key] = struct{}{}
	}
	return nil
}

func normalizeSkillSourceAddress(value string) string {
	path := strings.TrimSpace(value)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(path), "file://") {
		path = path[len("file://"):]
	}
	if !isLocalSkillSourceAddress(path) {
		return path
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				path = home
			} else {
				path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
			}
		}
	}
	if filepath.IsAbs(path) || isWindowsAbsSkillSourceAddress(path) {
		return filepath.Clean(path)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(absPath)
}

func isLocalSkillSourceAddress(value string) bool {
	switch {
	case strings.HasPrefix(strings.ToLower(value), "file://"):
		return true
	case strings.HasPrefix(value, "~"):
		return true
	case strings.HasPrefix(value, "./"), strings.HasPrefix(value, "../"):
		return true
	case filepath.IsAbs(value):
		return true
	case isWindowsAbsSkillSourceAddress(value):
		return true
	default:
		return false
	}
}

func isWindowsAbsSkillSourceAddress(value string) bool {
	if len(value) < 3 {
		return false
	}
	drive := value[0]
	if !((drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')) {
		return false
	}
	if value[1] != ':' {
		return false
	}
	return value[2] == '\\' || value[2] == '/'
}
