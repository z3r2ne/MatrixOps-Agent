package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"matrixops.local/core_agent/workersv2/builtin"
	database "pkgs/db"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// WorkerHandler Worker 处理器
type WorkerHandler struct {
	db *gorm.DB
}

type workerExportRequest struct {
	IDs   []uint   `json:"ids"`
	Names []string `json:"names"`
}

type workerBulkApplyConfigRequest struct {
	WorkerIDs         []uint `json:"workerIds"`
	Provider          string `json:"provider"`
	Model             string `json:"model"`
	ModelSettingsName string `json:"modelSettingsName"`
	LLMConfigID       *uint  `json:"llmConfigId"`
}

type workerYAML struct {
	Name              string      `yaml:"name"`
	Provider          string      `yaml:"provider"`
	Model             string      `yaml:"model"`
	ModelSettingsName string      `yaml:"modelSettingsName,omitempty"`
	Description       string      `yaml:"description,omitempty"`
	Mode              string      `yaml:"mode,omitempty"`
	Native            bool        `yaml:"native,omitempty"`
	Hidden            bool        `yaml:"hidden,omitempty"`
	TopP              float64     `yaml:"topP,omitempty"`
	Temperature       *float64     `yaml:"temperature"`
	Color             string      `yaml:"color,omitempty"`
	SystemPrompt      string      `yaml:"systemPrompt,omitempty"`
	Options           interface{} `yaml:"options,omitempty"`
	Steps             int         `yaml:"steps,omitempty"`
	EnabledTools      interface{} `yaml:"enabledTools,omitempty"`
	EnabledSkills     interface{} `yaml:"enabledSkills,omitempty"`
	Occupation        string      `yaml:"occupation,omitempty"`
	LLMConfigID       *uint       `yaml:"llmConfigId,omitempty"`
	WorkingDir        string      `yaml:"workingDir,omitempty"`
}

// NewWorkerHandler 创建 Worker 处理器
func NewWorkerHandler(db *gorm.DB) *WorkerHandler {
	return &WorkerHandler{db: db}
}

func (h *WorkerHandler) isProviderEnabled(provider string) (bool, error) {
	setting, err := database.GetProviderSettingByName(h.db, provider)
	if err != nil {
		return false, err
	}
	return setting.Enabled, nil
}

func (h *WorkerHandler) toWorkerResponse(worker models.Worker) models.WorkerResponse {
	provider := worker.Provider

	// 如果有 LLMConfigID，通过它查询对应的 LLM 配置
	if worker.LLMConfigID != nil {
		if llmConfig, err := database.GetLLMConfigByID(h.db, *worker.LLMConfigID); err == nil {
			provider = llmConfig.Name
		} else {
			// 如果查询失败，provider 设置为空
			provider = ""
		}
	}

	return models.WorkerResponse{
		ID:                worker.ID,
		Name:              worker.Name,
		Provider:          provider,
		Model:             worker.Model,
		ModelSettingsName: worker.ModelSettingsName,
		Description:       worker.Description,
		Temperature:       worker.Temperature,
		SystemPrompt:      worker.SystemPrompt,
		Occupation:        worker.Occupation,
		EnabledTools:      worker.EnabledTools,
		EnabledSkills:     worker.EnabledSkills,
		LLMConfigID:       worker.LLMConfigID,
		WorkingDir:        worker.WorkingDir,
		CreatedAt:         worker.CreatedAt,
		UpdatedAt:         worker.UpdatedAt,
	}
}

// GetWorkers 获取所有 Worker
func (h *WorkerHandler) GetWorkers(c *gin.Context) {
	workers, err := database.GetAllWorkers(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 Worker 失败"})
		return
	}

	results := make([]models.WorkerResponse, 0, len(workers))
	for _, w := range workers {
		results = append(results, h.toWorkerResponse(w))
	}

	c.JSON(http.StatusOK, results)
}

// CreateWorker 创建 Worker
func (h *WorkerHandler) CreateWorker(c *gin.Context) {
	var req models.WorkerCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	enabledToolsValue, err := h.normalizeEnabledToolsValue(req.EnabledTools)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enabledTools 格式错误"})
		return
	}
	enabledSkillsValue, err := h.normalizeEnabledSkillsValue(req.EnabledSkills)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enabledSkills 格式错误"})
		return
	}

	provider := strings.ToLower(req.Provider)
	if models.IsCompactionWorker(req.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "compaction worker 为系统内置，不能手动创建"})
		return
	}
	// 允许 llm 类型的 provider，或者跳过检查（现在使用大模型配置作为 Provider）
	if provider != "llm" {
		enabled, err := h.isProviderEnabled(provider)
		if err != nil || !enabled {
			// 如果不是内置 provider，检查是否是大模型配置的名称
			count, _ := database.CountLLMConfigsByName(h.db, provider)
			if count == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Provider 未启用或不存在"})
				return
			}
		}
	}

	worker := models.Worker{
		Name:              req.Name,
		Provider:          provider,
		Model:             req.Model,
		ModelSettingsName: strings.TrimSpace(req.ModelSettingsName),
		Description:       req.Description,
		Temperature:       req.Temperature,
		SystemPrompt:      req.SystemPrompt,
		Occupation:        req.Occupation,
		EnabledTools:      enabledToolsValue,
		EnabledSkills:     enabledSkillsValue,
		LLMConfigID:       req.LLMConfigID,
		WorkingDir:        req.WorkingDir,
	}
	if worker.ModelSettingsName == "" {
		worker.ModelSettingsName = database.DefaultModelSettingsName
	}

	if err := database.CreateWorker(h.db, &worker); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建 Worker 失败"})
		return
	}

	c.JSON(http.StatusCreated, h.toWorkerResponse(worker))
}

// UpdateWorker 更新 Worker
func (h *WorkerHandler) UpdateWorker(c *gin.Context) {
	id := c.Param("id")

	var wid uint
	fmt.Sscanf(id, "%d", &wid)

	worker, err := database.GetWorkerByID(h.db, wid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker 不存在"})
		return
	}

	var req models.WorkerUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	if req.Name != nil {
		if models.IsCompactionWorker(worker.Name) && !models.IsCompactionWorker(*req.Name) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能重命名 compaction worker"})
			return
		}
		if !models.IsCompactionWorker(worker.Name) && models.IsCompactionWorker(*req.Name) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能使用保留名称 compaction"})
			return
		}
		worker.Name = *req.Name
	}
	if req.Description != nil {
		worker.Description = *req.Description
	}
	if req.ModelSettingsName != nil {
		worker.ModelSettingsName = strings.TrimSpace(*req.ModelSettingsName)
	}
	if strings.TrimSpace(worker.ModelSettingsName) == "" {
		worker.ModelSettingsName = database.DefaultModelSettingsName
	}
	if req.Model != nil {
		worker.Model = *req.Model
	}
	if req.Temperature != nil {
		worker.Temperature = req.Temperature
	} else {
		worker.Temperature = nil
	}
	if req.SystemPrompt != nil {
		worker.SystemPrompt = *req.SystemPrompt
	}
	if req.Occupation != nil {
		worker.Occupation = *req.Occupation
	}
	if req.EnabledTools != nil {
		enabledToolsValue, err := h.normalizeEnabledToolsValue(*req.EnabledTools)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "enabledTools 格式错误"})
			return
		}
		worker.EnabledTools = enabledToolsValue
	}
	if req.EnabledSkills != nil {
		enabledSkillsValue, err := h.normalizeEnabledSkillsValue(*req.EnabledSkills)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "enabledSkills 格式错误"})
			return
		}
		worker.EnabledSkills = enabledSkillsValue
	}
	if req.LLMConfigID != nil {
		worker.LLMConfigID = req.LLMConfigID
	}
	if req.WorkingDir != nil {
		worker.WorkingDir = *req.WorkingDir
	}
	models.NormalizeCompactionWorkerFields(worker)
	if req.Provider != nil {
		provider := strings.ToLower(*req.Provider)
		if provider != "llm" {
			enabled, err := h.isProviderEnabled(provider)
			if err != nil || !enabled {
				// 如果不是内置 provider，检查是否是大模型配置的名称
				count, _ := database.CountLLMConfigsByName(h.db, provider)
				if count == 0 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Provider 未启用或不存在"})
					return
				}
			}
		}
		worker.Provider = provider
	}
	if err := database.UpdateWorker(h.db, worker); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新 Worker 失败"})
		return
	}

	c.JSON(http.StatusOK, h.toWorkerResponse(*worker))
}

// DeleteWorker 删除 Worker
func (h *WorkerHandler) DeleteWorker(c *gin.Context) {
	id := c.Param("id")

	var wid uint
	fmt.Sscanf(id, "%d", &wid)

	worker, err := database.GetWorkerByID(h.db, wid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker 不存在"})
		return
	}
	if models.IsCompactionWorker(worker.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除 compaction worker"})
		return
	}

	if err := database.DeleteWorker(h.db, wid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除 Worker 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Worker 删除成功"})
}

// BulkApplyConfig 批量应用 Worker 的 provider / model / model settings 配置
func (h *WorkerHandler) BulkApplyConfig(c *gin.Context) {
	var req workerBulkApplyConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	if len(req.WorkerIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少选择一个 Worker"})
		return
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "模型名不能为空"})
		return
	}

	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if req.LLMConfigID != nil {
		llmConfig, err := database.GetLLMConfigByID(h.db, *req.LLMConfigID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Provider 配置不存在"})
			return
		}
		provider = strings.ToLower(strings.TrimSpace(llmConfig.Name))
	}
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider 不能为空"})
		return
	}
	if provider != "llm" {
		enabled, err := h.isProviderEnabled(provider)
		if err != nil || !enabled {
			count, _ := database.CountLLMConfigsByName(h.db, provider)
			if count == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Provider 未启用或不存在"})
				return
			}
		}
	}

	modelSettingsName := strings.TrimSpace(req.ModelSettingsName)
	if modelSettingsName == "" {
		modelSettingsName = database.DefaultModelSettingsName
	}

	uniqueIDs := make([]uint, 0, len(req.WorkerIDs))
	seen := make(map[uint]struct{}, len(req.WorkerIDs))
	for _, id := range req.WorkerIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	if len(uniqueIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少选择一个有效 Worker"})
		return
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		for _, workerID := range uniqueIDs {
			worker, err := database.GetWorkerByID(tx, workerID)
			if err != nil {
				return fmt.Errorf("worker %d not found: %w", workerID, err)
			}
			worker.Provider = provider
			worker.Model = model
			worker.ModelSettingsName = modelSettingsName
			worker.LLMConfigID = req.LLMConfigID
			models.NormalizeCompactionWorkerFields(worker)
			if err := database.UpdateWorker(tx, worker); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "存在不存在的 Worker"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "批量应用 Worker 配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Worker 配置已批量应用",
		"updated": len(uniqueIDs),
	})
}

// RestoreDefaultWorkers 恢复默认 Workers
func (h *WorkerHandler) RestoreDefaultWorkers(c *gin.Context) {
	// 获取默认 LLM 配置
	defaultLLMConfig, err := database.GetDefaultLLMConfig(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "未找到默认 LLM 配置"})
		return
	}
	model := "gpt-5.4"
	// if defaultLLMConfig.Model != "" {
	// 	model = strings.Split(defaultLLMConfig.Model, ",")[0]
	// }
	// 执行初始化
	if err := database.RestoreBuiltInWorkers(h.db, model, defaultLLMConfig, builtin.ReadAll()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复默认 Workers 失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "默认 Workers 已恢复"})
}

// ExportWorkers 导出 Worker 配置（zip + yaml）
func (h *WorkerHandler) ExportWorkers(c *gin.Context) {
	var req workerExportRequest
	_ = c.ShouldBindJSON(&req)

	workers, err := database.GetAllWorkers(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 Worker 失败"})
		return
	}

	filtered := h.filterWorkersForExport(workers, req)
	if len(filtered) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未选择可导出的 Worker"})
		return
	}

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	usedNames := map[string]struct{}{}
	for _, worker := range filtered {
		filename := h.uniqueWorkerFilename(worker.Name, usedNames)
		entry, err := zipWriter.Create(filename)
		if err != nil {
			_ = zipWriter.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建导出文件失败"})
			return
		}
		yamlData, err := yaml.Marshal(h.workerToYAML(worker))
		if err != nil {
			_ = zipWriter.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "序列化 Worker 失败"})
			return
		}
		if _, err := entry.Write(yamlData); err != nil {
			_ = zipWriter.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "写入导出文件失败"})
			return
		}
	}
	if err := zipWriter.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成导出文件失败"})
		return
	}

	filename := fmt.Sprintf("workers-%s.zip", time.Now().Format("20060102-150405"))
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "application/zip", buffer.Bytes())
}

// ImportWorkers 导入 Worker 配置（zip + yaml）
func (h *WorkerHandler) ImportWorkers(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少导入文件"})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取导入文件失败"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取导入内容失败"})
		return
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "导入文件不是有效的 zip"})
		return
	}

	imported := 0
	updated := 0
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		fileReader, err := f.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(fileReader)
		fileReader.Close()
		if err != nil {
			continue
		}
		var payload workerYAML
		if err := yaml.Unmarshal(content, &payload); err != nil {
			continue
		}
		payload.Name = strings.TrimSpace(payload.Name)
		if payload.Name == "" {
			continue
		}
		provider := strings.ToLower(strings.TrimSpace(payload.Provider))
		if provider == "" {
			provider = "llm"
		}
		if provider != "llm" {
			enabled, err := h.isProviderEnabled(provider)
			if err != nil || !enabled {
				count, _ := database.CountLLMConfigsByName(h.db, provider)
				if count == 0 {
					continue
				}
			}
		}
		optionsValue, err := h.normalizeOptionsValue(payload.Options)
		if err != nil {
			continue
		}
		enabledToolsValue, err := h.normalizeEnabledToolsValue(payload.EnabledTools)
		if err != nil {
			continue
		}
		enabledSkillsValue, err := h.normalizeEnabledSkillsValue(payload.EnabledSkills)
		if err != nil {
			continue
		}
		worker := models.Worker{
			Name:              payload.Name,
			Provider:          provider,
			Model:             strings.TrimSpace(payload.Model),
			ModelSettingsName: strings.TrimSpace(payload.ModelSettingsName),
			Description:       strings.TrimSpace(payload.Description),
			Mode:              strings.TrimSpace(payload.Mode),
			Native:            payload.Native,
			Hidden:            payload.Hidden,
			TopP:              payload.TopP,
			Temperature:       payload.Temperature,
			Color:             strings.TrimSpace(payload.Color),
			SystemPrompt:      payload.SystemPrompt,
			Options:           optionsValue,
			Steps:             payload.Steps,
			Occupation:        payload.Occupation,
			EnabledTools:      enabledToolsValue,
			EnabledSkills:     enabledSkillsValue,
			LLMConfigID:       payload.LLMConfigID,
			WorkingDir:        payload.WorkingDir,
		}
		if worker.Model == "" {
			continue
		}
		if worker.ModelSettingsName == "" {
			worker.ModelSettingsName = database.DefaultModelSettingsName
		}
		if existing, err := database.GetWorkerByName(h.db, worker.Name); err == nil && existing != nil {
			worker.ID = existing.ID
			worker.CreatedAt = existing.CreatedAt
			if err := database.UpdateWorker(h.db, &worker); err == nil {
				updated++
			}
			continue
		}
		if err := database.CreateWorker(h.db, &worker); err == nil {
			imported++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Worker 导入完成",
		"imported": imported,
		"updated":  updated,
	})
}

func (h *WorkerHandler) workerToYAML(worker models.Worker) workerYAML {
	var enabledToolsValue interface{}
	var enabledSkillsValue interface{}
	var optionsValue interface{}
	if strings.TrimSpace(worker.EnabledTools) != "" {
		var arr []string
		if err := json.Unmarshal([]byte(worker.EnabledTools), &arr); err == nil {
			enabledToolsValue = arr
		} else {
			enabledToolsValue = worker.EnabledTools
		}
	}
	if strings.TrimSpace(worker.EnabledSkills) != "" {
		var arr []string
		if err := json.Unmarshal([]byte(worker.EnabledSkills), &arr); err == nil {
			enabledSkillsValue = arr
		} else {
			enabledSkillsValue = worker.EnabledSkills
		}
	}
	if strings.TrimSpace(worker.Options) != "" {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(worker.Options), &obj); err == nil {
			optionsValue = obj
		} else {
			optionsValue = worker.Options
		}
	}
	return workerYAML{
		Name:              worker.Name,
		Provider:          worker.Provider,
		Model:             worker.Model,
		ModelSettingsName: worker.ModelSettingsName,
		Description:       worker.Description,
		Mode:              worker.Mode,
		Native:            worker.Native,
		Hidden:            worker.Hidden,
		TopP:              worker.TopP,
		Temperature:       worker.Temperature,
		Color:             worker.Color,
		SystemPrompt:      worker.SystemPrompt,
		Options:           optionsValue,
		Steps:             worker.Steps,
		EnabledTools:      enabledToolsValue,
		EnabledSkills:     enabledSkillsValue,
		Occupation:        worker.Occupation,
		LLMConfigID:       worker.LLMConfigID,
		WorkingDir:        worker.WorkingDir,
	}
}

func (h *WorkerHandler) normalizeEnabledToolsValue(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	switch typed := value.(type) {
	case string:
		return typed, nil
	case []string:
		return models.NormalizeEnabledToolsJSON(typed), nil
	case []interface{}:
		names := make([]string, 0, len(typed))
		for _, item := range typed {
			name, ok := item.(string)
			if !ok {
				continue
			}
			names = append(names, name)
		}
		return models.NormalizeEnabledToolsJSON(names), nil
	default:
		return "", fmt.Errorf("invalid enabled tools format")
	}
}

func (h *WorkerHandler) normalizeEnabledSkillsValue(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	switch typed := value.(type) {
	case string:
		return typed, nil
	case []string:
		return models.NormalizeEnabledSkillsJSON(typed), nil
	case []interface{}:
		names := make([]string, 0, len(typed))
		for _, item := range typed {
			name, ok := item.(string)
			if !ok {
				continue
			}
			names = append(names, name)
		}
		return models.NormalizeEnabledSkillsJSON(names), nil
	default:
		return "", fmt.Errorf("invalid enabled skills format")
	}
}

func (h *WorkerHandler) normalizeOptionsValue(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	switch typed := value.(type) {
	case string:
		return typed, nil
	case map[string]interface{}:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return "", err
		}
		return string(encoded), nil
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return "", err
		}
		return string(encoded), nil
	}
}

func (h *WorkerHandler) filterWorkersForExport(workers []models.Worker, req workerExportRequest) []models.Worker {
	if len(req.IDs) == 0 && len(req.Names) == 0 {
		return workers
	}
	ids := map[uint]struct{}{}
	for _, id := range req.IDs {
		ids[id] = struct{}{}
	}
	names := map[string]struct{}{}
	for _, name := range req.Names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			names[trimmed] = struct{}{}
		}
	}
	filtered := []models.Worker{}
	for _, worker := range workers {
		if _, ok := ids[worker.ID]; ok {
			filtered = append(filtered, worker)
			continue
		}
		if _, ok := names[worker.Name]; ok {
			filtered = append(filtered, worker)
		}
	}
	return filtered
}

func (h *WorkerHandler) uniqueWorkerFilename(name string, used map[string]struct{}) string {
	base := strings.TrimSpace(name)
	if base == "" {
		base = "worker"
	}
	base = h.sanitizeFilename(base)
	if base == "" {
		base = "worker"
	}
	filename := base + ".yaml"
	if _, ok := used[filename]; !ok {
		used[filename] = struct{}{}
		return filename
	}
	idx := 2
	for {
		candidate := fmt.Sprintf("%s-%d.yaml", base, idx)
		if _, ok := used[candidate]; !ok {
			used[candidate] = struct{}{}
			return candidate
		}
		idx++
	}
}

func (h *WorkerHandler) sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "..", "")
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	name = replacer.Replace(name)
	return strings.Trim(name, " -")
}
