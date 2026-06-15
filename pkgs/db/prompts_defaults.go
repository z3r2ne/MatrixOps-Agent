package database

import (
	"strings"

	"pkgs/promptdefs"

	"gorm.io/gorm"
)

func EnsurePromptDefaults(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	globalPrompt, err := GetGlobalPrompt(db)
	if err != nil {
		return err
	}
	if strings.TrimSpace(globalPrompt) == "" {
		if err := SetGlobalPrompt(db, promptdefs.DefaultGlobalPrompt()); err != nil {
			return err
		}
	}

	occupations, err := GetAllOccupations(db)
	if err != nil {
		return err
	}
	for _, occupation := range occupations {
		if strings.TrimSpace(occupation.Prompt) != "" {
			continue
		}
		defaultPrompt := promptdefs.DefaultOccupationPrompt(occupation.Code)
		if strings.TrimSpace(defaultPrompt) == "" {
			continue
		}
		if err := UpdateOccupation(db, occupation.ID, map[string]interface{}{
			"prompt": defaultPrompt,
		}); err != nil {
			return err
		}
	}

	return nil
}
