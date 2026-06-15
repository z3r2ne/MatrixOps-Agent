package session

import (
	"sort"
	"strings"

	"matrixops-agent/types"
	"pkgs/db/models"
	"pkgs/db/storage"
	"pkgs/skillfs"

	"gorm.io/gorm"
)

func buildWorkerSkillPrompts(raw string) ([]types.FilePrompt, error) {
	enabledSkills, hasEnabledSkills, err := models.ParseEnabledSkills(raw)
	if err != nil {
		return nil, err
	}
	if !hasEnabledSkills || len(enabledSkills) == 0 {
		return nil, nil
	}

	names := make([]string, 0, len(enabledSkills))
	for name := range enabledSkills {
		names = append(names, name)
	}
	sort.Strings(names)

	prompts := make([]types.FilePrompt, 0, len(names))
	for _, name := range names {
		skill, content, err := skillfs.LoadInstalledSkillContent(name)
		if err != nil {
			continue
		}
		prompts = append(prompts, types.FilePrompt{
			Path:   "skill://" + skill.Name,
			Prompt: content,
		})
	}
	return prompts, nil
}

func ensureWorkerSkillsInSession(db *gorm.DB, sessionID string, worker *models.Worker) error {
	if db == nil || worker == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}
	enabledSkills, hasEnabledSkills, err := models.ParseEnabledSkills(worker.EnabledSkills)
	if err != nil || !hasEnabledSkills || len(enabledSkills) == 0 {
		return err
	}
	_, err = storage.UpdateSessionByCallback(db, sessionID, func(info *types.Info) error {
		if info == nil {
			return nil
		}
		for name := range enabledSkills {
			skill, _, loadErr := skillfs.LoadInstalledSkillContent(name)
			if loadErr != nil {
				continue
			}
			exists := false
			for _, enabled := range info.EnabledSkills {
				if strings.EqualFold(strings.TrimSpace(enabled), skill.Name) {
					exists = true
					break
				}
			}
			if !exists {
				info.EnabledSkills = append(info.EnabledSkills, skill.Name)
			}
		}
		return nil
	})
	return err
}
