package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	"matrixops-agent/ilink"
	"matrixops/services"
	database "pkgs/db"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ILinkHandler iLink 微信账号处理器
type ILinkHandler struct {
	db      *gorm.DB
	service *services.ILinkService
}

// NewILinkHandler 创建处理器
func NewILinkHandler(db *gorm.DB, service *services.ILinkService) *ILinkHandler {
	return &ILinkHandler{db: db, service: service}
}

// GetAccounts 获取所有微信账号（同一用户仅保留最新一条）
func (h *ILinkHandler) GetAccounts(c *gin.Context) {
	h.service.SyncAccounts()

	var accounts []models.WechatAccount
	if err := h.db.Order("updated_at DESC").Find(&accounts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取账号列表失败"})
		return
	}
	c.JSON(http.StatusOK, h.applyRuntimeStatus(h.dedupeAndPruneAccounts(accounts)))
}

// GetAccount 获取单个账号
func (h *ILinkHandler) GetAccount(c *gin.Context) {
	id := c.Param("id")
	var account models.WechatAccount
	if err := h.db.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账号不存在"})
		return
	}
	c.JSON(http.StatusOK, account)
}

// DeleteAccount 删除账号
func (h *ILinkHandler) DeleteAccount(c *gin.Context) {
	id := c.Param("id")
	var account models.WechatAccount
	if err := h.db.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账号不存在"})
		return
	}

	// 如果正在运行，先停止
	if h.service.IsAccountRunning(account.BotID) {
		_ = h.service.StopAccount(account.BotID)
	}

	if err := h.db.Delete(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除账号失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "账号已删除"})
}

// UpdateAccount 更新账号（启用状态、任务绑定）
func (h *ILinkHandler) UpdateAccount(c *gin.Context) {
	id := c.Param("id")
	var account models.WechatAccount
	if err := h.db.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账号不存在"})
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	var req models.WechatAccountUpdate
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	var raw map[string]json.RawMessage
	_ = json.Unmarshal(body, &raw)

	updates := map[string]interface{}{}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	boundTaskInRequest := false
	var boundTaskID *uint
	if rawBound, ok := raw["boundTaskId"]; ok {
		boundTaskInRequest = true
		if string(rawBound) == "null" {
			boundTaskID = nil
		} else if req.BoundTaskID != nil {
			boundTaskID = req.BoundTaskID
			updates["bound_task_id"] = *req.BoundTaskID
		}
	} else if req.BoundTaskID != nil {
		boundTaskInRequest = true
		boundTaskID = req.BoundTaskID
		updates["bound_task_id"] = *req.BoundTaskID
	}

	if len(updates) > 0 {
		if err := h.db.Model(&account).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新账号失败"})
			return
		}
	}

	h.db.First(&account, id)

	if boundTaskInRequest {
		_ = h.service.UpdateAccountBinding(account.BotID, boundTaskID)
		h.db.First(&account, id)
	}

	// 处理启停
	if req.Enabled != nil {
		if *req.Enabled && !h.service.IsAccountRunning(account.BotID) {
			go h.service.StartAccount(&account)
		} else if !*req.Enabled && h.service.IsAccountRunning(account.BotID) {
			_ = h.service.StopAccount(account.BotID)
		}
	}

	c.JSON(http.StatusOK, account)
}

// FetchQRCode 获取登录二维码
func (h *ILinkHandler) FetchQRCode(c *gin.Context) {
	resp, err := ilink.FetchQRCode(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// PollQRStatus 轮询扫码状态
func (h *ILinkHandler) PollQRStatus(c *gin.Context) {
	qrcode := c.Query("qrcode")
	if qrcode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 qrcode 参数"})
		return
	}

	creds, err := ilink.PollQRStatus(c.Request.Context(), qrcode, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	account, err := h.upsertAccountFromLogin(creds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存账号失败"})
		return
	}

	go h.service.StartAccount(account)

	c.JSON(http.StatusOK, account)
}

// StartAccount 手动启动账号
func (h *ILinkHandler) StartAccount(c *gin.Context) {
	id := c.Param("id")
	var account models.WechatAccount
	if err := h.db.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账号不存在"})
		return
	}
	if err := h.service.StartAccount(&account); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "账号已启动"})
}

// StopAccount 手动停止账号
func (h *ILinkHandler) StopAccount(c *gin.Context) {
	id := c.Param("id")
	var account models.WechatAccount
	if err := h.db.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账号不存在"})
		return
	}
	if err := h.service.StopAccount(account.BotID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "账号已停止"})
}

// GetTasksForBinding 获取当前工作区内可用于绑定的会话（任务）列表
func (h *ILinkHandler) GetTasksForBinding(c *gin.Context) {
	workspaceIDStr := c.Query("workspaceId")
	if workspaceIDStr == "" {
		c.JSON(http.StatusOK, []models.Task{})
		return
	}
	wid, err := strconv.Atoi(workspaceIDStr)
	if err != nil || wid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的工作区 ID"})
		return
	}
	tasks, err := database.GetTasksWithProjectByWorkspaceID(h.db, uint(wid))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取任务列表失败"})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

func wechatAccountDedupeKey(acc models.WechatAccount) string {
	if acc.ILinkUserID != "" {
		return acc.ILinkUserID
	}
	return acc.BotID
}

func (h *ILinkHandler) applyRuntimeStatus(accounts []models.WechatAccount) []models.WechatAccount {
	for i := range accounts {
		if h.service.IsAccountRunning(accounts[i].BotID) {
			accounts[i].Status = "online"
		} else if accounts[i].Status == "online" {
			accounts[i].Status = "offline"
		}
	}
	return accounts
}

// dedupeAndPruneAccounts 按 iLink 用户 ID（无则按 BotID）去重，保留最新记录并删除旧记录。
func (h *ILinkHandler) dedupeAndPruneAccounts(accounts []models.WechatAccount) []models.WechatAccount {
	if len(accounts) <= 1 {
		return accounts
	}

	keepers := make(map[string]models.WechatAccount)
	var toDelete []models.WechatAccount
	for _, acc := range accounts {
		key := wechatAccountDedupeKey(acc)
		if _, exists := keepers[key]; exists {
			toDelete = append(toDelete, acc)
			continue
		}
		keepers[key] = acc
	}

	for _, dup := range toDelete {
		_ = h.service.StopAccount(dup.BotID)
		_ = h.db.Delete(&dup).Error
	}

	result := make([]models.WechatAccount, 0, len(keepers))
	for _, acc := range keepers {
		result = append(result, acc)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

// upsertAccountFromLogin 扫码登录后写入账号：同一用户用新凭证覆盖旧记录。
func (h *ILinkHandler) upsertAccountFromLogin(creds *ilink.Credentials) (*models.WechatAccount, error) {
	var existing []models.WechatAccount
	query := h.db
	if creds.ILinkUserID != "" {
		query = query.Where(
			h.db.Where(&models.WechatAccount{ILinkUserID: creds.ILinkUserID}).
				Or(&models.WechatAccount{BotID: creds.ILinkBotID}),
		)
	} else {
		query = query.Where("bot_id = ?", creds.ILinkBotID)
	}
	if err := query.Order("updated_at DESC").Find(&existing).Error; err != nil {
		return nil, err
	}

	if len(existing) == 0 {
		account := models.WechatAccount{
			BotID:       creds.ILinkBotID,
			BotToken:    creds.BotToken,
			BaseURL:     creds.BaseURL,
			ILinkUserID: creds.ILinkUserID,
			Status:      "offline",
			Enabled:     true,
		}
		if err := h.db.Create(&account).Error; err != nil {
			return nil, err
		}
		return &account, nil
	}

	account := existing[0]
	for i := 1; i < len(existing); i++ {
		dup := existing[i]
		_ = h.service.StopAccount(dup.BotID)
		_ = h.db.Delete(&dup).Error
	}

	oldBotID := account.BotID
	_ = h.service.StopAccount(oldBotID)

	account.BotID = creds.ILinkBotID
	account.BotToken = creds.BotToken
	account.BaseURL = creds.BaseURL
	account.ILinkUserID = creds.ILinkUserID
	account.Status = "offline"
	account.Enabled = true
	if err := h.db.Save(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}
