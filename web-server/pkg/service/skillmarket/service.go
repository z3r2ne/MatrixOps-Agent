package skillmarket

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	database "pkgs/db"
	"pkgs/db/models"

	"gopkg.in/yaml.v3"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) SyncSource(source *models.SkillSource) ([]models.SkillCard, error) {
	if source == nil {
		return nil, errors.New("skill source is nil")
	}
	localPath, err := database.SkillSourceLocalPath(source.ID)
	if err != nil {
		return nil, err
	}
	source.LocalPath = localPath

	if err := ensureSourceCheckout(localPath, strings.TrimSpace(source.RepoURL)); err != nil {
		now := time.Now()
		source.LastSyncAt = &now
		source.LastSyncError = err.Error()
		source.SkillCount = 0
		return nil, err
	}

	skills, err := s.scanSourceSkills(*source)
	now := time.Now()
	source.LastSyncAt = &now
	if err != nil {
		source.LastSyncError = err.Error()
		source.SkillCount = 0
		return nil, err
	}
	source.LastSyncError = ""
	source.SkillCount = len(skills)
	return skills, nil
}

func (s *Service) ListSkills(sources []models.SkillSource, installedOnly bool) ([]models.SkillCard, error) {
	out := make([]models.SkillCard, 0)
	for _, source := range sources {
		skills, err := s.scanSourceSkills(source)
		if err != nil {
			continue
		}
		for _, skill := range skills {
			if installedOnly && !skill.Installed {
				continue
			}
			out = append(out, skill)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Installed != out[j].Installed {
			return out[i].Installed && !out[j].Installed
		}
		if out[i].SourceName != out[j].SourceName {
			return out[i].SourceName < out[j].SourceName
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *Service) InstallSkill(source models.SkillSource, relativePath string) (string, error) {
	sourceRoot, err := sourceSkillsRoot(source)
	if err != nil {
		return "", err
	}
	relativePath = cleanRelativeSkillPath(relativePath)
	skillDir := filepath.Join(sourceRoot, relativePath)
	if !isSkillDir(skillDir) {
		return "", fmt.Errorf("skill not found: %s", relativePath)
	}
	targetPath, err := database.InstalledSkillPath(source.ID, relativePath)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(targetPath); err != nil {
		return "", err
	}
	if err := copyDir(skillDir, targetPath); err != nil {
		return "", err
	}
	return targetPath, nil
}

func (s *Service) UninstallSkill(sourceID uint, relativePath string) error {
	targetPath, err := database.InstalledSkillPath(sourceID, cleanRelativeSkillPath(relativePath))
	if err != nil {
		return err
	}
	if err := os.RemoveAll(targetPath); err != nil {
		return err
	}
	return nil
}

func (s *Service) scanSourceSkills(source models.SkillSource) ([]models.SkillCard, error) {
	sourceRoot, err := sourceSkillsRoot(source)
	if err != nil {
		return nil, err
	}
	cards := make([]models.SkillCard, 0)
	err = filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			if path != sourceRoot && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != "SKILL.md" {
			return nil
		}
		skillDir := filepath.Dir(path)
		rel, err := filepath.Rel(sourceRoot, skillDir)
		if err != nil {
			return nil
		}
		rel = cleanRelativeSkillPath(rel)
		meta, err := parseSkillMetadata(path)
		if err != nil {
			return nil
		}
		installedPath, err := database.InstalledSkillPath(source.ID, rel)
		if err != nil {
			return nil
		}
		installed := isSkillDir(installedPath)
		cards = append(cards, models.SkillCard{
			ID:            fmt.Sprintf("%d:%s", source.ID, rel),
			SourceID:      source.ID,
			SourceName:    source.Name,
			SourceEnabled: source.Enabled,
			Name:          meta.Name,
			Description:   meta.Description,
			RelativePath:  rel,
			Installed:     installed,
			InstalledPath: installedPath,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cards, nil
}

type skillMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func parseSkillMetadata(skillFile string) (skillMetadata, error) {
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return skillMetadata{}, err
	}
	text := string(data)
	meta := skillMetadata{
		Name: filepath.Base(filepath.Dir(skillFile)),
	}
	if strings.HasPrefix(text, "---\n") {
		if rest := strings.TrimPrefix(text, "---\n"); rest != text {
			if idx := strings.Index(rest, "\n---"); idx >= 0 {
				frontMatter := rest[:idx]
				var parsed skillMetadata
				if err := yaml.Unmarshal([]byte(frontMatter), &parsed); err == nil {
					if strings.TrimSpace(parsed.Name) != "" {
						meta.Name = strings.TrimSpace(parsed.Name)
					}
					if strings.TrimSpace(parsed.Description) != "" {
						meta.Description = strings.TrimSpace(parsed.Description)
					}
				}
				text = rest[idx+4:]
			}
		}
	}
	if strings.TrimSpace(meta.Description) == "" {
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") {
				continue
			}
			meta.Description = line
			break
		}
	}
	return meta, nil
}

func sourceSkillsRoot(source models.SkillSource) (string, error) {
	localPath := strings.TrimSpace(source.LocalPath)
	if localPath == "" {
		var err error
		localPath, err = database.SkillSourceLocalPath(source.ID)
		if err != nil {
			return "", err
		}
	}
	skillsPath := strings.TrimSpace(source.SkillsPath)
	if skillsPath == "" {
		skillsPath = "skills"
	}
	root := filepath.Join(localPath, filepath.Clean(skillsPath))
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("skills path is not a directory: %s", root)
	}
	return root, nil
}

func cleanRelativeSkillPath(relativePath string) string {
	cleaned := filepath.Clean(strings.TrimSpace(relativePath))
	cleaned = strings.TrimPrefix(cleaned, string(filepath.Separator))
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func isSkillDir(path string) bool {
	info, err := os.Stat(filepath.Join(path, "SKILL.md"))
	return err == nil && !info.IsDir()
}

func ensureSourceCheckout(localPath, repoURL string) error {
	if database.IsBuiltInSkillSource(repoURL) {
		return nil
	}
	if isLocalSourceAddress(repoURL) {
		return syncLocalSource(localPath, repoURL)
	}
	parent := filepath.Dir(localPath)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	if !isGitCheckout(localPath) {
		_ = os.RemoveAll(localPath)
		return runGit(parent, "clone", "--depth", "1", repoURL, localPath)
	}
	currentURL, _ := gitOutput(localPath, "config", "--get", "remote.origin.url")
	if strings.TrimSpace(currentURL) != "" && strings.TrimSpace(currentURL) != strings.TrimSpace(repoURL) {
		_ = os.RemoveAll(localPath)
		return runGit(parent, "clone", "--depth", "1", repoURL, localPath)
	}
	if err := runGit(localPath, "fetch", "--all", "--prune"); err != nil {
		return err
	}
	return runGit(localPath, "pull", "--ff-only")
}

func syncLocalSource(localPath, sourceAddress string) error {
	sourcePath, err := resolveLocalSourcePath(sourceAddress)
	if err != nil {
		return err
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("local source path is not a directory: %s", sourcePath)
	}
	parent := filepath.Dir(localPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	if filepath.Clean(sourcePath) == filepath.Clean(localPath) {
		return fmt.Errorf("local source path cannot equal cache path: %s", sourcePath)
	}
	if err := os.RemoveAll(localPath); err != nil {
		return err
	}
	return copyDir(sourcePath, localPath)
}

func resolveLocalSourcePath(sourceAddress string) (string, error) {
	path := strings.TrimSpace(sourceAddress)
	if path == "" {
		return "", errors.New("skill source address is empty")
	}
	if strings.HasPrefix(strings.ToLower(path), "file://") {
		path = path[len("file://"):]
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	if !filepath.IsAbs(path) && !isWindowsAbsPath(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		path = absPath
	}
	return filepath.Clean(path), nil
}

func isLocalSourceAddress(repoURL string) bool {
	value := strings.TrimSpace(repoURL)
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	switch {
	case strings.HasPrefix(lower, "file://"):
		return true
	case strings.HasPrefix(value, "~"):
		return true
	case strings.HasPrefix(value, "./"), strings.HasPrefix(value, "../"):
		return true
	case filepath.IsAbs(value):
		return true
	case isWindowsAbsPath(value):
		return true
	default:
		return false
	}
}

func isWindowsAbsPath(path string) bool {
	if len(path) < 3 {
		return false
	}
	drive := path[0]
	if !((drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')) {
		return false
	}
	if path[1] != ':' {
		return false
	}
	return path[2] == '\\' || path[2] == '/'
}

func isGitCheckout(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info != nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}
