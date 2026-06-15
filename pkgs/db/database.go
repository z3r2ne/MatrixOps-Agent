package database

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"pkgs/db/models"
	"pkgs/shellutil"

	"gorm.io/gorm"
)

var DB *gorm.DB

// DBPath returns the shared database path.
func DBPath() (string, error) {
	dataDir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "matrixops.db"), nil
}

// InitDB 初始化数据库连接。
// builtInWorkerYAML 为内置 Worker 的 YAML（路径 key -> 内容）；可为 nil 表示不种子化 Worker。
// builtInSkillFiles 为内置技能的文件（如 research/SKILL.md -> 内容）；可为 nil 表示不安装内置技能。
func InitDB(db *gorm.DB, builtInWorkerYAML map[string][]byte, builtInSkillFiles map[string][]byte) error {

	// 自动迁移
	if err := migrateMemorySearchSchema(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(
		&models.Workspace{},
		&models.Project{},
		&models.MemoryLibrary{},
		&models.ProviderSetting{},
		&models.Worker{},
		&models.Task{},
		&models.TaskExecution{},
		&models.ExecutionLog{},
		&models.CommandLog{},    // 系统命令执行日志
		&models.LLMConfig{},     // 大模型配置
		&models.ModelSettings{}, // 模型设置
		&models.GlobalConfig{},  // 全局配置
		&models.Occupation{},    // 职业配置
		&models.SkillSource{},   // Skill 源
		&models.McpServer{},     // MCP 服务器
		&models.SearchConfig{},  // 搜索插件配置
		&models.EmbeddingConfig{}, // 本地 embedding 配置
		&models.MemorySearchDocument{}, // 记忆检索索引元数据
		&models.MemoryLibraryIndexJob{}, // 记忆库检索索引任务
		&models.ReminderJob{},
		&models.Session{},
		&models.Message{},
		&models.Part{},
		&models.MessagePromptSnapshot{},
		&models.MessageCodeSnapshot{},
		&models.MemoryEntry{},
		&models.Plan{},                  // 会话计划
		&models.OpenUIApplicationItem{}, // 桌面端已打开工作区/项目
		&models.UIState{},               // 桌面端 UI 状态
		&models.WechatAccount{},         // iLink 微信账号
	); err != nil {
		return err
	}
	if err := finishMemorySearchSchemaMigration(db); err != nil {
		return err
	}

	// 旧库升级：worktree_path 列迁移后可能为 ''，与 path 对齐（避免 NOT NULL 无 default 的迁移失败依赖 default:''）
	if err := db.Exec("UPDATE projects SET worktree_path = path WHERE TRIM(COALESCE(worktree_path, '')) = ''").Error; err != nil {
		return fmt.Errorf("回填 project.worktree_path: %w", err)
	}
	if err := db.Exec("UPDATE llm_configs SET api_type = ? WHERE TRIM(COALESCE(api_type, '')) = ''", models.LLMAPITypeResponse).Error; err != nil {
		return fmt.Errorf("回填 llm_configs.api_type: %w", err)
	}
	if err := db.Exec("UPDATE llm_configs SET system_prompt_placement = ? WHERE TRIM(COALESCE(system_prompt_placement, '')) = ''", "instruction").Error; err != nil {
		return fmt.Errorf("回填 llm_configs.system_prompt_placement: %w", err)
	}
	if err := db.Exec("UPDATE messages SET memory = NULL WHERE memory IS NOT NULL").Error; err != nil {
		return fmt.Errorf("清理 message.memory: %w", err)
	}
	if db.Migrator().HasColumn(&models.Part{}, "memory") {
		if err := db.Migrator().DropColumn(&models.Part{}, "memory"); err != nil {
			return fmt.Errorf("删除 parts.memory: %w", err)
		}
	}
	if db.Migrator().HasColumn(&models.Part{}, "prompt") {
		if err := db.Migrator().DropColumn(&models.Part{}, "prompt"); err != nil {
			return fmt.Errorf("删除 parts.prompt: %w", err)
		}
	}
	if db.Migrator().HasColumn(&models.ModelSettings{}, "memory_type") {
		if err := db.Migrator().DropColumn(&models.ModelSettings{}, "memory_type"); err != nil {
			return fmt.Errorf("删除 model_settings.memory_type: %w", err)
		}
	}
	if err := EnsureDefaultModelSettings(db); err != nil {
		return fmt.Errorf("确保默认模型配置: %w", err)
	}
	if err := db.Exec("UPDATE workers SET model_settings_name = ? WHERE TRIM(COALESCE(model_settings_name, '')) = ''", DefaultModelSettingsName).Error; err != nil {
		return fmt.Errorf("回填 worker.model_settings_name: %w", err)
	}
	if err := BackfillTaskWorkspaceIDs(db); err != nil {
		return fmt.Errorf("回填 task.workspace_id: %w", err)
	}
	if err := fixLegacyMemoryLibraryIDsJSON(db); err != nil {
		return fmt.Errorf("修复 memory_library_ids JSON: %w", err)
	}
	if err := db.Exec("UPDATE workspaces SET group_mode = ? WHERE TRIM(COALESCE(group_mode, '')) = ''", models.DefaultTaskListGroupMode).Error; err != nil {
		return fmt.Errorf("回填 workspace.group_mode: %w", err)
	}

	// 初始化默认配置
	var count int64
	db.Model(&models.GlobalConfig{}).Where("key = ?", models.ConfigKeyKeepProcessAlive).Count(&count)
	if count == 0 {
		// 默认关闭常驻进程
		db.Create(&models.GlobalConfig{
			Key:   models.ConfigKeyKeepProcessAlive,
			Value: "false",
		})
	}
	db.Model(&models.GlobalConfig{}).Where("key = ?", models.ConfigKeyDefaultShell).Count(&count)
	if count == 0 {
		defaultShell, customShell := shellutil.NormalizeForInit("", "")
		db.Create(&models.GlobalConfig{
			Key:   models.ConfigKeyDefaultShell,
			Value: defaultShell,
		})
		if defaultShell == shellutil.ShellCustom && customShell != "" {
			db.Model(&models.GlobalConfig{}).Where("key = ?", models.ConfigKeyCustomShellCommand).Count(&count)
			if count == 0 {
				db.Create(&models.GlobalConfig{
					Key:   models.ConfigKeyCustomShellCommand,
					Value: customShell,
				})
			}
		}
	}
	db.Model(&models.GlobalConfig{}).Where("key = ?", models.ConfigKeyCustomShellCommand).Count(&count)
	if count == 0 {
		db.Create(&models.GlobalConfig{
			Key:   models.ConfigKeyCustomShellCommand,
			Value: "",
		})
	}
	db.Model(&models.GlobalConfig{}).Where("key = ?", models.ConfigKeyAgentMaxSteps).Count(&count)
	if count == 0 {
		db.Create(&models.GlobalConfig{
			Key:   models.ConfigKeyAgentMaxSteps,
			Value: fmt.Sprintf("%d", models.DefaultAgentMaxSteps),
		})
	}
	db.Model(&models.GlobalConfig{}).Where("key = ?", models.ConfigKeyMemoryCompactionTriggerThresholdPercent).Count(&count)
	if count == 0 {
		db.Create(&models.GlobalConfig{
			Key:   models.ConfigKeyMemoryCompactionTriggerThresholdPercent,
			Value: fmt.Sprintf("%d", models.DefaultMemoryCompactionTriggerThresholdPercent),
		})
	}
	db.Model(&models.GlobalConfig{}).Where("key = ?", models.ConfigKeyMemoryCompactionTargetPercent).Count(&count)
	if count == 0 {
		db.Create(&models.GlobalConfig{
			Key:   models.ConfigKeyMemoryCompactionTargetPercent,
			Value: fmt.Sprintf("%d", models.DefaultMemoryCompactionTargetPercent),
		})
	}
	db.Model(&models.GlobalConfig{}).Where("key = ?", models.ConfigKeyMemoryCompactionL2ScopePercent).Count(&count)
	if count == 0 {
		db.Create(&models.GlobalConfig{
			Key:   models.ConfigKeyMemoryCompactionL2ScopePercent,
			Value: fmt.Sprintf("%d", models.DefaultMemoryCompactionL2ScopePercent),
		})
	}
	db.Model(&models.GlobalConfig{}).Where("key = ?", models.ConfigKeyDefaultTaskListGroupMode).Count(&count)
	if count == 0 {
		db.Create(&models.GlobalConfig{
			Key:   models.ConfigKeyDefaultTaskListGroupMode,
			Value: string(models.DefaultTaskListGroupMode),
		})
	}
	db.Model(&models.GlobalConfig{}).Where("key = ?", models.ConfigKeyDefaultProjectToolPermissions).Count(&count)
	if count == 0 {
		db.Create(&models.GlobalConfig{
			Key:   models.ConfigKeyDefaultProjectToolPermissions,
			Value: "{}",
		})
	}

	DB = db
	log.Println("数据库初始化成功")

	// 清理异常状态的任务
	if err := CleanupStaleActiveTasks(db); err != nil {
		log.Printf("清理异常任务状态失败: %v", err)
		// 不返回错误，继续初始化
	}
	if err := BackfillSubtaskParentTaskIDs(db); err != nil {
		log.Printf("回填子任务父任务关系失败: %v", err)
	}

	if err := InitDefaultConfigs(db, builtInWorkerYAML); err != nil {
		return err
	}
	if err := EnsureDefaultSkillSources(db); err != nil {
		return fmt.Errorf("初始化默认技能源: %w", err)
	}
	if err := InitBuiltInSkills(db, builtInSkillFiles); err != nil {
		return fmt.Errorf("初始化内置技能: %w", err)
	}
	if err := EnsurePromptDefaults(db); err != nil {
		return fmt.Errorf("初始化默认提示词: %w", err)
	}
	if err := ensureDefaultShellConfig(db); err != nil {
		return err
	}
	return nil
}

// GetDB 获取数据库实例
// func GetDB() *gorm.DB {
// 	return DB
// }

// SetDB 设置数据库实例（主要用于测试）
func SetDB(db *gorm.DB) {
	DB = db
}

func InitDefaultConfigs(db *gorm.DB, builtInWorkerYAML map[string][]byte) error {
	log.Println("初始化默认配置")

	// 初始化默认职业
	log.Println("初始化默认职业")
	if err := InitDefaultOccupations(db); err != nil {
		log.Printf("初始化默认职业失败: %v", err)
		return err
	}

	// 获取llm配置数量
	var count int64
	db.Model(&models.LLMConfig{}).Count(&count)
	if count == 0 {
		log.Println("初始化默认 LLM 配置")
		if err := InitDefaultLLMConfigs(db); err != nil {
			return err
		}
	}

	// 检查默认 LLM 配置是否有效
	if err := ensureDefaultLLMConfig(db); err != nil {
		log.Printf("确保默认 LLM 配置失败: %v", err)
		return err
	}

	// 获取默认LLM配置
	defaultLLMConfig, err := GetDefaultLLMConfig(db)
	if err != nil {
		log.Printf("获取默认 LLM 配置失败: %v", err)
		return err
	}

	log.Println("初始化默认 Worker 配置")
	if err := InitBuiltInWorkers(db, "gpt-5.4", defaultLLMConfig, builtInWorkerYAML); err != nil {
		return err
	}

	// 获取worker数量
	// var workerCount int64
	// db.Model(&models.Worker{}).Count(&workerCount)
	// if workerCount == 0 {
	// 	log.Println("初始化默认 Worker 配置")
	// 	if err := InitBuiltInWorkers(db, "gpt-5.2-codex", defaultLLMConfig, builtInWorkerYAML); err != nil {
	// 		return err
	// 	}
	// }

	return nil
}

// ensureDefaultLLMConfig 确保默认 LLM 配置有效
func ensureDefaultLLMConfig(db *gorm.DB) error {
	// 尝试获取默认配置 ID
	defaultID, err := GetDefaultLLMConfigID(db)
	if err != nil {
		// 如果没有设置默认配置，获取第一个配置并设置为默认
		log.Println("未找到默认 LLM 配置，尝试设置第一个配置为默认")
		var firstConfig models.LLMConfig
		if err := db.First(&firstConfig).Error; err != nil {
			return fmt.Errorf("没有可用的 LLM 配置: %w", err)
		}
		return SetDefaultLLMConfigID(db, firstConfig.ID)
	}

	// 检查默认配置是否存在
	if _, err := GetLLMConfigByID(db, defaultID); err != nil {
		log.Printf("默认 LLM 配置 (ID=%d) 不存在，重新设置默认配置", defaultID)
		var firstConfig models.LLMConfig
		if err := db.First(&firstConfig).Error; err != nil {
			return fmt.Errorf("没有可用的 LLM 配置: %w", err)
		}
		return SetDefaultLLMConfigID(db, firstConfig.ID)
	}

	return nil
}

func ensureDefaultShellConfig(db *gorm.DB) error {
	defaultShellConfig, err := GetGlobalConfigByKey(db, models.ConfigKeyDefaultShell)
	if err != nil || strings.TrimSpace(defaultShellConfig.Value) == "" {
		selectedShell, customShell := shellutil.NormalizeForInit("", "")
		if err := UpsertGlobalConfig(db, models.ConfigKeyDefaultShell, selectedShell); err != nil {
			return err
		}
		if selectedShell == shellutil.ShellCustom && customShell != "" {
			if err := UpsertGlobalConfig(db, models.ConfigKeyCustomShellCommand, customShell); err != nil {
				return err
			}
		}
	}
	return nil
}

func fixLegacyMemoryLibraryIDsJSON(db *gorm.DB) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "sqlite" {
		return nil
	}
	for _, table := range []string{"tasks", "projects"} {
		if err := db.Exec(`
UPDATE ` + table + `
SET memory_library_ids = json_array(CAST(memory_library_ids AS INTEGER))
WHERE memory_library_ids IS NOT NULL
  AND TRIM(memory_library_ids) != ''
  AND TRIM(memory_library_ids) != 'null'
  AND json_valid(memory_library_ids)
  AND json_type(memory_library_ids) = 'integer'`).Error; err != nil {
			return fmt.Errorf("%s: %w", table, err)
		}
	}
	return nil
}
