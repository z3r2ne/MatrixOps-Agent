package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	agentllm "matrixops-agent/llm"
	agentprovider "matrixops-agent/provider"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/httpclient"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// LLMHandler 大模型相关操作处理器
type LLMHandler struct {
	db *gorm.DB
}

type previewLLMModelsRequest struct {
	Type    string `json:"type" binding:"required"`
	APIKey  string `json:"apiKey" binding:"required"`
	BaseURL string `json:"baseUrl"`
	Proxy   string `json:"proxy"`
}

// NewLLMHandler 创建 LLM 处理器
func NewLLMHandler(db *gorm.DB) *LLMHandler {
	return &LLMHandler{db: db}
}

// GetLLMConfigs 获取所有大模型配置
func (h *LLMHandler) GetLLMConfigs(c *gin.Context) {
	configs, err := database.GetAllLLMConfigs(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, configs)
}

// GetLLMModels 获取大模型支持的模型列表
func (h *LLMHandler) GetLLMModels(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "配置 ID 不能为空"})
		return
	}

	var cid uint
	fmt.Sscanf(id, "%d", &cid)

	config, err := database.GetLLMConfigByID(h.db, cid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	modelNames, statusCode, err := fetchLLMModels(*config, nil)
	if err != nil {
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"models": modelNames})
}

// PreviewLLMModels 使用未保存的配置预览模型列表
func (h *LLMHandler) PreviewLLMModels(c *gin.Context) {
	var req previewLLMModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	modelNames, statusCode, err := fetchLLMModels(models.LLMConfig{
		Type:    strings.TrimSpace(req.Type),
		APIKey:  strings.TrimSpace(req.APIKey),
		BaseURL: strings.TrimSpace(req.BaseURL),
		Proxy:   strings.TrimSpace(req.Proxy),
	}, nil)
	if err != nil {
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"models": modelNames})
}

func resolveLLMModelsBaseURL(config models.LLMConfig) (string, error) {
	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" {
		switch config.Type {
		case "openai":
			baseURL = "https://api.openai.com/v1"
		case "claude":
			baseURL = "https://api.anthropic.com/v1"
		case "custom":
			return "", fmt.Errorf("自定义模型必须配置 BaseURL")
		default:
			return "", fmt.Errorf("不支持的模型类型: %s", config.Type)
		}
	}
	return baseURL, nil
}

func fetchLLMModels(config models.LLMConfig, client *http.Client) ([]string, int, error) {
	baseURL, err := resolveLLMModelsBaseURL(config)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	url := fmt.Sprintf("%s/models", strings.TrimSuffix(baseURL, "/"))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("创建请求失败: %w", err)
	}

	if config.Type == "openai" || config.Type == "custom" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	} else if config.Type == "claude" {
		req.Header.Set("x-api-key", config.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	if client == nil {
		client = llmHTTPClient(config)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("API 请求失败 (%d): %s", resp.StatusCode, string(body))
	}

	modelNames, err := extractLLMModelNames(body)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("解析响应失败: %w", err)
	}
	return modelNames, http.StatusOK, nil
}

func extractLLMModelNames(body []byte) ([]string, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	modelNames := make([]string, 0, 32)
	seen := map[string]struct{}{}
	appendName := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		modelNames = append(modelNames, name)
	}

	appendFromArray := func(items []interface{}) {
		for _, item := range items {
			switch typed := item.(type) {
			case string:
				appendName(typed)
			case map[string]interface{}:
				if id, ok := typed["id"].(string); ok {
					appendName(id)
				}
				if name, ok := typed["name"].(string); ok {
					appendName(name)
				}
			}
		}
	}

	if data, ok := result["data"].([]interface{}); ok {
		appendFromArray(data)
	}
	if modelsArr, ok := result["models"].([]interface{}); ok {
		appendFromArray(modelsArr)
	}

	return modelNames, nil
}

// GetLLMConfig 获取单个大模型配置
func (h *LLMHandler) GetLLMConfig(c *gin.Context) {
	id := c.Param("id")

	var cid uint
	fmt.Sscanf(id, "%d", &cid)

	config, err := database.GetLLMConfigByID(h.db, cid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// CreateLLMConfig 创建大模型配置
func (h *LLMHandler) CreateLLMConfig(c *gin.Context) {
	var req models.LLMConfigCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	config := models.LLMConfig{
		Name:                  req.Name,
		Type:                  req.Type,
		APIKey:                req.APIKey,
		Model:                 req.Model,
		BaseURL:               req.BaseURL,
		APIType:               models.NormalizeLLMAPIType(req.APIType),
		Proxy:                 req.Proxy,
		MaxRetries:            maxLLMConfigRetries(req.MaxRetries),
		NativeOpenAIToolCalls: req.NativeOpenAIToolCalls,
	}

	if err := database.CreateLLMConfig(h.db, &config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, config)
}

// UpdateLLMConfig 更新大模型配置
func (h *LLMHandler) UpdateLLMConfig(c *gin.Context) {
	id := c.Param("id")

	var cid uint
	fmt.Sscanf(id, "%d", &cid)

	if _, err := database.GetLLMConfigByID(h.db, cid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	var req models.LLMConfigUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 更新字段
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.APIKey != nil {
		updates["api_key"] = *req.APIKey
	}
	if req.Model != nil {
		updates["model"] = *req.Model
	}
	if req.BaseURL != nil {
		updates["base_url"] = *req.BaseURL
	}
	if req.APIType != nil {
		updates["api_type"] = models.NormalizeLLMAPIType(*req.APIType)
	}
	if req.Proxy != nil {
		updates["proxy"] = *req.Proxy
	}
	if req.MaxRetries != nil {
		updates["max_retries"] = maxLLMConfigRetries(*req.MaxRetries)
	}
	if req.NativeOpenAIToolCalls != nil {
		updates["native_open_ai_tool_calls"] = *req.NativeOpenAIToolCalls
	}

	if err := database.UpdateLLMConfigFields(h.db, cid, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败: " + err.Error()})
		return
	}

	updatedConfig, err := database.GetLLMConfigByID(h.db, cid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取更新后的配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, updatedConfig)
}

// DeleteLLMConfig 删除大模型配置
func (h *LLMHandler) DeleteLLMConfig(c *gin.Context) {
	id := c.Param("id")

	var cid uint
	fmt.Sscanf(id, "%d", &cid)

	if _, err := database.GetLLMConfigByID(h.db, cid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	if err := database.DeleteLLMConfig(h.db, cid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置已删除"})
}

func maxLLMConfigRetries(value int) int {
	if value <= 0 {
		return 5
	}
	return value
}

func llmHTTPClient(cfg models.LLMConfig) *http.Client {
	if c := httpclient.ClientWithOptionalProxy(cfg.Proxy); c != nil {
		return c
	}
	return &http.Client{}
}

// GenerateCommitMessage 使用大模型生成提交消息
func (h *LLMHandler) GenerateCommitMessage(c *gin.Context) {
	var req models.GenerateCommitMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 获取配置
	var config *models.LLMConfig
	var err error
	if req.ConfigID != nil {
		config, err = database.GetLLMConfigByID(h.db, *req.ConfigID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
			return
		}
	} else {
		// 使用默认配置
		configs, err := database.GetAllLLMConfigs(h.db)
		if err != nil || len(configs) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到默认配置，请先在设置中配置大模型"})
			return
		}
		// 获取默认配置
		config, err = database.GetDefaultLLMConfig(h.db)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到默认配置，请先在设置中配置大模型"})
			return
		}
	}

	// 调用大模型生成提交消息
	message, err := h.callLLM(*config, req.Diff)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成提交消息失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.GenerateCommitMessageResponse{
		Message: message,
	})
}

// DebugLLM 调试调用大模型（单轮）
func (h *LLMHandler) DebugLLM(c *gin.Context) {
	var req models.DebugLLMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	input := strings.TrimSpace(req.Input)
	if input == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "输入内容不能为空"})
		return
	}

	// 获取配置
	var config *models.LLMConfig
	var err error
	if req.ConfigID != nil {
		config, err = database.GetLLMConfigByID(h.db, *req.ConfigID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
			return
		}
	} else {
		config, err = database.GetDefaultLLMConfig(h.db)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到默认配置，请先在设置中配置大模型"})
			return
		}
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = firstModelName(config.Model)
	}
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "模型名称不能为空"})
		return
	}

	temperature := 0.7
	if req.Temperature != nil {
		temperature = *req.Temperature
		if temperature < 0 {
			temperature = 0
		} else if temperature > 2 {
			temperature = 2
		}
	}

	maxTokens := 512
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}

	systemPrompt := "你是一个有帮助的助手，请简洁、准确地回答用户。"
	message, err := h.callLLMWithParams(*config, model, systemPrompt, input, temperature, maxTokens)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "调试调用失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.DebugLLMResponse{
		Text: message,
	})
}

// callLLM 调用大模型 API
func (h *LLMHandler) callLLM(config models.LLMConfig, diff string) (string, error) {
	// 构建提示词
	systemPrompt := "你是一个专业的代码提交消息生成助手。请根据提供的 git diff 内容，生成一个简洁、清晰、专业的提交消息。提交消息应该：1. 使用中文 2. 简洁明了，一句话概括主要变更 3. 不超过 50 个字符 4. 只返回提交消息本身，不要有任何其他内容或解释"
	userPrompt := fmt.Sprintf("请为以下代码变更生成一个提交消息：\n\n```\n%s\n```", diff)

	return h.callLLMWithParams(config, config.Model, systemPrompt, userPrompt, 0.7, 100)
}

func firstModelName(modelList string) string {
	for _, item := range strings.Split(modelList, ",") {
		name := strings.TrimSpace(item)
		if name != "" {
			return name
		}
	}
	return strings.TrimSpace(modelList)
}

// callLLMWithParams 调用大模型 API（支持参数）
func (h *LLMHandler) callLLMWithParams(config models.LLMConfig, model, systemPrompt, userPrompt string, temperature float64, maxTokens int) (string, error) {
	if model == "" {
		model = config.Model
	}
	if temperature < 0 {
		temperature = 0
	} else if temperature > 2 {
		temperature = 2
	}
	if maxTokens <= 0 {
		maxTokens = 256
	}

	switch config.Type {
	case "openai":
		return h.callOpenAI(config, model, systemPrompt, userPrompt, temperature, maxTokens)
	case "claude":
		return h.callClaude(config, model, systemPrompt, userPrompt, temperature, maxTokens)
	case "custom":
		return h.callOpenAI(config, model, systemPrompt, userPrompt, temperature, maxTokens) // 自定义通常兼容 OpenAI API
	default:
		return "", fmt.Errorf("不支持的模型类型: %s", config.Type)
	}
}

// callOpenAI 调用 OpenAI API
func (h *LLMHandler) callOpenAI(config models.LLMConfig, model, systemPrompt, userPrompt string, temperature float64, maxTokens int) (string, error) {
	client := agentprovider.NewGenericClient()
	resp, err := client.Chat(agentllm.ChatRequest{
		Context: context.Background(),
		Messages: []*agentllm.ModelMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature:     temperature,
		MaxOutputTokens: maxTokens,
		ProviderOptions: &config,
		Model:           model,
	})
	if err != nil {
		return "", err
	}
	if content, ok := resp.Message.Content.(string); ok {
		return strings.TrimSpace(content), nil
	}
	if payload, err := json.Marshal(resp.Message.Content); err == nil {
		return strings.TrimSpace(string(payload)), nil
	}
	return strings.TrimSpace(fmt.Sprint(resp.Message.Content)), nil
}

// callClaude 调用 Claude API
func (h *LLMHandler) callClaude(config models.LLMConfig, model, systemPrompt, userPrompt string, temperature float64, maxTokens int) (string, error) {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	url := fmt.Sprintf("%s/messages", strings.TrimSuffix(baseURL, "/"))

	reqBody := map[string]interface{}{
		"model":      model,
		"max_tokens": maxTokens,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	}
	if temperature >= 0 {
		reqBody["temperature"] = temperature
	}

	jsonData, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := llmHTTPClient(config)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API 请求失败 (%d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		return "", fmt.Errorf("API 响应格式错误")
	}

	contentBlock := content[0].(map[string]interface{})
	text := contentBlock["text"].(string)

	return strings.TrimSpace(text), nil
}

// GetDefaultLLMConfig 获取默认 LLM 配置
func (h *LLMHandler) GetDefaultLLMConfig(c *gin.Context) {
	config, err := database.GetDefaultLLMConfig(h.db)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到默认配置: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, config)
}

// SetDefaultLLMConfig 设置默认 LLM 配置
func (h *LLMHandler) SetDefaultLLMConfig(c *gin.Context) {
	id := c.Param("id")
	var cid uint
	if _, err := fmt.Sscanf(id, "%d", &cid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的配置 ID"})
		return
	}

	if err := database.SetDefaultLLMConfigID(h.db, cid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设置默认配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "默认配置设置成功"})
}
