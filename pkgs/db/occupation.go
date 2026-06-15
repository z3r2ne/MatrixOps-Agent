package database

import (
	"errors"
	"pkgs/db/models"

	"gorm.io/gorm"
)

// GetAllOccupations 获取所有职业
func GetAllOccupations(db *gorm.DB) ([]models.Occupation, error) {
	var occupations []models.Occupation
	if err := db.Order("code ASC").Find(&occupations).Error; err != nil {
		return nil, err
	}
	return occupations, nil
}

// GetOccupationByID 根据ID获取职业
func GetOccupationByID(db *gorm.DB, id uint) (*models.Occupation, error) {
	var occupation models.Occupation
	if err := db.First(&occupation, id).Error; err != nil {
		return nil, err
	}
	return &occupation, nil
}

// GetOccupationByCode 根据代码获取职业
func GetOccupationByCode(db *gorm.DB, code string) (*models.Occupation, error) {
	var occupation models.Occupation
	if err := db.Where("code = ?", code).First(&occupation).Error; err != nil {
		return nil, err
	}
	return &occupation, nil
}

func GetOccupationByName(db *gorm.DB, name string) (*models.Occupation, error) {
	var occupation models.Occupation
	if err := db.Where("name = ?", name).First(&occupation).Error; err != nil {
		return nil, err
	}
	return &occupation, nil
}

// CreateOccupation 创建职业
func CreateOccupation(db *gorm.DB, occupation *models.Occupation) error {
	return db.Create(occupation).Error
}

// UpdateOccupation 更新职业
func UpdateOccupation(db *gorm.DB, id uint, updates map[string]interface{}) error {
	return db.Model(&models.Occupation{}).Where("id = ?", id).Updates(updates).Error
}
func UpdateOccupationInstance(db *gorm.DB, occupation *models.Occupation) error {
	return db.Model(&models.Occupation{}).Where("code = ?", occupation.Code).Save(occupation).Error
}

// DeleteOccupation 删除职业
func DeleteOccupation(db *gorm.DB, id uint) error {
	return db.Delete(&models.Occupation{}, id).Error
}

// InitDefaultOccupations 初始化默认职业
func InitDefaultOccupations(db *gorm.DB) error {
	occupations := []models.Occupation{
		{
			Code:        "analyst",
			Name:        "分析师",
			Description: "专注于分析用户需求，制定解决方案，协调其他 Worker",
			Color:       "cyan",
			Prompt:      "",
		},
		{
			Code:        "coder",
			Name:        "研发工程师",
			Description: "专注于编写代码，实现功能",
			Color:       "blue",
			Prompt:      "",
		},
		{
			Code:        "reviewer",
			Name:        "验收师",
			Description: "专注于代码审查，质量验收",
			Color:       "purple",
			Prompt:      "",
		},
		{
			Code:        "orchestrator",
			Name:        "指挥师",
			Description: "专注于任务编排，流程控制",
			Color:       "orange",
			Prompt:      "",
		},
		{
			Code:        "planner",
			Name:        "规划师",
			Description: "专注于项目规划，架构设计",
			Color:       "green",
			Prompt:      "",
		},
	}

	for _, occ := range occupations {
		// 检查是否已存在
		var existing models.Occupation
		err := db.Where("code = ?", occ.Code).First(&existing).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 不存在，创建
				if err := CreateOccupation(db, &occ); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	return nil
}

// GetGlobalPrompt 获取全局提示词
func GetGlobalPrompt(db *gorm.DB) (string, error) {
	config, err := GetGlobalConfigByKey(db, models.ConfigKeyGlobalPrompt)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return config.Value, nil
}

// SetGlobalPrompt 设置全局提示词
func SetGlobalPrompt(db *gorm.DB, prompt string) error {
	return UpsertGlobalConfig(db, models.ConfigKeyGlobalPrompt, prompt)
}
