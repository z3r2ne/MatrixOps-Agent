package skillfs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	database "pkgs/db"

	"gopkg.in/yaml.v3"
)

type InstalledSkill struct {
	Name         string
	Description  string
	Path         string
	RelativePath string
}

func ListInstalledSkills() ([]InstalledSkill, error) {
	root, err := database.InstalledSkillsDir()
	if err != nil {
		return nil, err
	}
	out := make([]InstalledSkill, 0)
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != "SKILL.md" {
			return nil
		}
		skill, err := parseInstalledSkill(root, filepath.Dir(path))
		if err != nil {
			return nil
		}
		out = append(out, skill)
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].RelativePath < out[j].RelativePath
	})
	return out, nil
}

func FindInstalledSkillByName(name string) (*InstalledSkill, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("skill name is required")
	}
	skills, err := ListInstalledSkills()
	if err != nil {
		return nil, err
	}
	var matches []InstalledSkill
	for _, skill := range skills {
		if strings.EqualFold(strings.TrimSpace(skill.Name), name) {
			matches = append(matches, skill)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("skill not found: %s", name)
	case 1:
		return &matches[0], nil
	default:
		paths := make([]string, 0, len(matches))
		for _, item := range matches {
			paths = append(paths, item.RelativePath)
		}
		return nil, fmt.Errorf("multiple skills named %q found: %s", name, strings.Join(paths, ", "))
	}
}

func LoadInstalledSkillContent(name string) (*InstalledSkill, string, error) {
	skill, err := FindInstalledSkillByName(name)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(filepath.Join(skill.Path, "SKILL.md"))
	if err != nil {
		return nil, "", err
	}
	return skill, string(data), nil
}

type skillFrontMatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func parseInstalledSkill(root, skillDir string) (InstalledSkill, error) {
	skillFile := filepath.Join(skillDir, "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return InstalledSkill{}, err
	}
	text := string(data)
	meta := skillFrontMatter{
		Name: filepath.Base(skillDir),
	}
	body := text
	if strings.HasPrefix(text, "---\n") {
		if rest := strings.TrimPrefix(text, "---\n"); rest != text {
			if idx := strings.Index(rest, "\n---"); idx >= 0 {
				frontMatter := rest[:idx]
				var parsed skillFrontMatter
				if err := yaml.Unmarshal([]byte(frontMatter), &parsed); err == nil {
					if strings.TrimSpace(parsed.Name) != "" {
						meta.Name = strings.TrimSpace(parsed.Name)
					}
					if strings.TrimSpace(parsed.Description) != "" {
						meta.Description = strings.TrimSpace(parsed.Description)
					}
				}
				body = rest[idx+4:]
			}
		}
	}
	if strings.TrimSpace(meta.Description) == "" {
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") {
				continue
			}
			meta.Description = line
			break
		}
	}
	rel, err := filepath.Rel(root, skillDir)
	if err != nil {
		return InstalledSkill{}, err
	}
	return InstalledSkill{
		Name:         strings.TrimSpace(meta.Name),
		Description:  strings.TrimSpace(meta.Description),
		Path:         skillDir,
		RelativePath: strings.TrimPrefix(filepath.Clean(rel), string(filepath.Separator)),
	}, nil
}
