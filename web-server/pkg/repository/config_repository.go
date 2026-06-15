package repository

import (
	"pkgs/db/models"

	"gorm.io/gorm"
)

// ConfigRepository 配置数据访问接口
type ConfigRepository interface {
	GetByKey(key string) (*models.GlobalConfig, error)
	Set(key, value string) error
	GetKeepProcessAlive() (bool, error)
	SetKeepProcessAlive(enabled bool) error
}

type configRepository struct {
	db *gorm.DB
}

// NewConfigRepository 创建配置仓储实例
func NewConfigRepository(db *gorm.DB) ConfigRepository {
	return &configRepository{db: db}
}

func (r *configRepository) GetByKey(key string) (*models.GlobalConfig, error) {
	var config models.GlobalConfig
	err := r.db.Where("key = ?", key).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *configRepository) Set(key, value string) error {
	var config models.GlobalConfig
	err := r.db.Where("key = ?", key).First(&config).Error
	if err == gorm.ErrRecordNotFound {
		// 创建新配置
		config = models.GlobalConfig{Key: key, Value: value}
		return r.db.Create(&config).Error
	} else if err != nil {
		return err
	}

	// 更新现有配置
	config.Value = value
	return r.db.Save(&config).Error
}

func (r *configRepository) GetKeepProcessAlive() (bool, error) {
	config, err := r.GetByKey(models.ConfigKeyKeepProcessAlive)
	if err != nil {
		return false, err
	}
	return config.Value == "true", nil
}

func (r *configRepository) SetKeepProcessAlive(enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	return r.Set(models.ConfigKeyKeepProcessAlive, value)
}
