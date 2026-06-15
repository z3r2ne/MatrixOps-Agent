package tool

import (
	"errors"
	"fmt"
	"strings"

	"matrixops-agent/types"
	"pkgs/db/storage"
	"pkgs/skillfs"

	"gorm.io/gorm"
)

type LoadSkillTool struct {
	db *gorm.DB
}

func (LoadSkillTool) Name() string {
	return "load_skill"
}

func (LoadSkillTool) VerbosName() string {
	return "加载技能"
}

func (LoadSkillTool) Description() string {
	return "按技能名称加载已安装技能的完整内容；完整正文会写入本次工具输出并进入对话历史，无需等待下一轮"
}

func (LoadSkillTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"name": map[string]interface{}{
			"type":        "string",
			"description": "The installed skill name to load",
		},
	}, []string{"name"})
}

func (t LoadSkillTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	name, _ := input["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return Result{IsError: true}, errors.New("load_skill: missing name")
	}
	skill, content, err := skillfs.LoadInstalledSkillContent(name)
	if err != nil {
		return Result{IsError: true}, err
	}
	if strings.TrimSpace(ctx.SessionID) == "" {
		return Result{IsError: true}, errors.New("load_skill: session id is required")
	}
	if t.db == nil {
		return Result{IsError: true}, errors.New("load_skill: database is required")
	}
	if _, err := storage.UpdateSessionByCallback(t.db, ctx.SessionID, func(info *types.Info) error {
		if info == nil {
			return nil
		}
		for _, enabled := range info.EnabledSkills {
			if strings.EqualFold(strings.TrimSpace(enabled), skill.Name) {
				return nil
			}
		}
		info.EnabledSkills = append(info.EnabledSkills, skill.Name)
		return nil
	}); err != nil {
		return Result{IsError: true}, err
	}

	body := strings.TrimSpace(content)
	if body == "" {
		body = "(empty skill document)"
	}
	fullText := fmt.Sprintf("[Skill: %s]\n%s", skill.Name, body)

	return Result{
		Name:               "load_skill",
		Content:            fullText,
		FullContent:        fullText,
		PreserveFullOutput: true,
		Metadata: map[string]interface{}{
			"skillName":         skill.Name,
			"skillPath":         skill.Path,
			"skillRelativePath": skill.RelativePath,
			"description":       skill.Description,
			"enabledInSession":  true,
			"preserveFullOutput": true,
		},
	}, nil
}
