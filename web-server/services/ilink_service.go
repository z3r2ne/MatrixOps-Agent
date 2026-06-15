package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"matrixops-agent/ilink"
	"matrixops-agent/types"
	"matrixops/services/task_runner"
	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/gorm"
)

// ILinkService 管理微信账号生命周期与消息双向转发
type ILinkService struct {
	db       *gorm.DB
	mu       sync.RWMutex
	accounts map[string]*ilinkAccount // botID -> account runtime
	hub      *GlobalWSHub
}

type ilinkAccount struct {
	bot            *ilink.WechatClawbot
	cancel         context.CancelFunc
	account        *models.WechatAccount
	listenerID     string
	lastFromUserID string
	lastCtxToken   string
	sessionExpired bool
	msgMu          sync.Mutex
	typingMu       sync.Mutex
	typingCancel   context.CancelFunc
}

var (
	ilinkService     *ILinkService
	ilinkServiceOnce sync.Once
)

// GetILinkService 获取单例
func GetILinkService(db *gorm.DB, hub *GlobalWSHub) *ILinkService {
	ilinkServiceOnce.Do(func() {
		ilinkService = &ILinkService{
			db:       db,
			accounts: make(map[string]*ilinkAccount),
			hub:      hub,
		}
	})
	return ilinkService
}

// LoadAndStartEnabledAccounts 从数据库加载所有启用的账号并启动监听
func (s *ILinkService) LoadAndStartEnabledAccounts() {
	s.SyncAccounts()
}

// IsAccountRunning 判断账号是否已在内存中运行
func (s *ILinkService) IsAccountRunning(botID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.accounts[botID]
	return ok
}

// SyncAccounts 对齐数据库与运行时：启用但未运行的账号会被启动，状态与运行时一致。
func (s *ILinkService) SyncAccounts() {
	var accounts []models.WechatAccount
	if err := s.db.Find(&accounts).Error; err != nil {
		log.Printf("[ilink] sync failed to load accounts: %v", err)
		return
	}

	for i := range accounts {
		acc := accounts[i]
		running := s.IsAccountRunning(acc.BotID)

		switch {
		case acc.Enabled && !running:
			log.Printf("[ilink] sync: starting enabled account %s (db status=%s)", acc.BotID, acc.Status)
			go s.StartAccount(&acc)
		case !acc.Enabled && running:
			log.Printf("[ilink] sync: stopping disabled account %s", acc.BotID)
			_ = s.StopAccount(acc.BotID)
		case !running && acc.Status != "offline" && acc.Status != "error":
			log.Printf("[ilink] sync: fixing stale status for %s (%s -> offline)", acc.BotID, acc.Status)
			s.db.Model(&acc).Update("status", "offline")
		case running && acc.Status != "online":
			s.db.Model(&acc).Update("status", "online")
		}
	}
}

// StartAccount 启动单个账号的消息监听
func (s *ILinkService) StartAccount(acc *models.WechatAccount) error {
	s.mu.Lock()
	if _, ok := s.accounts[acc.BotID]; ok {
		s.mu.Unlock()
		return nil
	}

	creds := &ilink.Credentials{
		BotToken:    acc.BotToken,
		ILinkBotID:  acc.BotID,
		BaseURL:     acc.BaseURL,
		ILinkUserID: acc.ILinkUserID,
	}
	bot := ilink.NewWechatClawbot(creds)
	botID := acc.BotID
	bot.OnFatalSessionExpired = func() {
		s.HandleSessionExpired(botID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runtime := &ilinkAccount{
		bot:     bot,
		cancel:  cancel,
		account: acc,
	}
	s.accounts[acc.BotID] = runtime
	s.mu.Unlock()

	// 注册任务输出监听器
	if acc.BoundTaskID != nil && *acc.BoundTaskID > 0 {
		s.registerTaskListener(runtime, *acc.BoundTaskID)
	}

	// 启动消息监听 goroutine
	go func() {
		log.Printf("[ilink] starting monitor for account %s", botID)
		err := bot.StartReceiving(ctx, s.makeMessageHandler(runtime))
		if err != nil && ctx.Err() == nil {
			log.Printf("[ilink] monitor stopped for account %s with error: %v", botID, err)
		} else {
			log.Printf("[ilink] monitor stopped for account %s", botID)
		}
		s.handleMonitorStopped(botID)
	}()

	// 更新状态
	s.db.Model(acc).Update("status", "online")
	return nil
}

func (s *ILinkService) removeRuntime(botID string) *ilinkAccount {
	s.mu.Lock()
	defer s.mu.Unlock()

	rt, ok := s.accounts[botID]
	if !ok {
		return nil
	}
	delete(s.accounts, botID)
	rt.stopTypingLoop()

	if rt.account.BoundTaskID != nil && *rt.account.BoundTaskID > 0 && rt.listenerID != "" {
		s.hub.UnregisterTaskMessageListener(*rt.account.BoundTaskID, rt.listenerID)
	}
	return rt
}

// HandleSessionExpired 会话无法自动恢复：停止监听、禁用账号并通知前端重新扫码登录。
func (s *ILinkService) HandleSessionExpired(botID string) {
	s.mu.Lock()
	rt, running := s.accounts[botID]
	if running && rt != nil {
		rt.sessionExpired = true
	}
	s.mu.Unlock()

	if running && rt != nil {
		rt.cancel()
		s.removeRuntime(botID)
	}

	var acc models.WechatAccount
	if err := s.db.Where("bot_id = ?", botID).First(&acc).Error; err != nil {
		log.Printf("[ilink] session expired for %s but account not found: %v", botID, err)
		return
	}

	if err := s.db.Model(&acc).Updates(map[string]interface{}{
		"enabled": false,
		"status":  "error",
	}).Error; err != nil {
		log.Printf("[ilink] failed to disable account %s after session expiry: %v", botID, err)
		return
	}

	if s.hub != nil {
		s.hub.BroadcastILinkSessionExpired(acc.ID, acc.BotID, acc.ILinkUserID)
	}
	log.Printf("[ilink] account %s session expired; disabled until re-login", botID)
}

func (s *ILinkService) handleMonitorStopped(botID string) {
	rt := s.removeRuntime(botID)
	if rt == nil {
		return
	}

	if rt.sessionExpired {
		return
	}

	var acc models.WechatAccount
	if err := s.db.Where("bot_id = ?", botID).First(&acc).Error; err != nil {
		return
	}

	if !acc.Enabled {
		s.db.Model(&acc).Update("status", "offline")
		return
	}

	log.Printf("[ilink] account %s monitor stopped while enabled, restarting", botID)
	s.db.Model(&acc).Update("status", "offline")
	time.AfterFunc(2*time.Second, func() {
		if s.IsAccountRunning(botID) {
			return
		}
		var latest models.WechatAccount
		if err := s.db.Where("bot_id = ?", botID).First(&latest).Error; err != nil || !latest.Enabled {
			return
		}
		_ = s.StartAccount(&latest)
	})
}

// StopAccount 停止单个账号的监听
func (s *ILinkService) StopAccount(botID string) error {
	rt := s.removeRuntime(botID)
	if rt == nil {
		return nil
	}

	rt.cancel()
	s.db.Model(&models.WechatAccount{}).Where("bot_id = ?", botID).Update("status", "offline")
	log.Printf("[ilink] stopped account %s", botID)
	return nil
}

// RestartAccount 重启账号
func (s *ILinkService) RestartAccount(acc *models.WechatAccount) error {
	_ = s.StopAccount(acc.BotID)
	return s.StartAccount(acc)
}

// UpdateAccountBinding 更新账号的任务绑定（数据库 + 运行时内存 + 出站监听器）。
func (s *ILinkService) UpdateAccountBinding(botID string, taskID *uint) error {
	s.mu.Lock()
	rt, running := s.accounts[botID]
	s.mu.Unlock()

	if running {
		if rt.account.BoundTaskID != nil && *rt.account.BoundTaskID > 0 && rt.listenerID != "" {
			s.hub.UnregisterTaskMessageListener(*rt.account.BoundTaskID, rt.listenerID)
			rt.listenerID = ""
		}
	}

	updates := map[string]interface{}{}
	if taskID != nil && *taskID > 0 {
		updates["bound_task_id"] = *taskID
	} else {
		updates["bound_task_id"] = nil
	}
	if err := s.db.Model(&models.WechatAccount{}).Where("bot_id = ?", botID).Updates(updates).Error; err != nil {
		return err
	}

	if !running || rt == nil {
		return nil
	}

	// 入站消息 handler 读取 rt.account.BoundTaskID，必须与数据库保持一致。
	if taskID != nil && *taskID > 0 {
		tid := *taskID
		rt.account.BoundTaskID = &tid
		s.mu.Lock()
		s.registerTaskListener(rt, tid)
		s.mu.Unlock()
	} else {
		rt.account.BoundTaskID = nil
	}
	return nil
}

// makeMessageHandler 构造微信消息处理器
func (s *ILinkService) makeMessageHandler(rt *ilinkAccount) ilink.MessageHandler {
	return func(ctx context.Context, client *ilink.Client, msg ilink.WeixinMessage) {
		if msg.MessageType != ilink.MessageTypeUser || msg.MessageState != ilink.MessageStateFinish {
			return
		}
		if rt.account.BoundTaskID == nil || *rt.account.BoundTaskID == 0 {
			return
		}

		inbound, err := ilink.ParseInboundMessage(ctx, msg)
		if err != nil {
			log.Printf("[ilink] parse inbound message failed: %v", err)
			return
		}
		if !inbound.HasContent() {
			return
		}

		rt.msgMu.Lock()
		rt.lastFromUserID = msg.FromUserID
		rt.lastCtxToken = msg.ContextToken
		rt.msgMu.Unlock()

		taskID := *rt.account.BoundTaskID
		content := inbound.Text
		workDir := ""
		if task, err := database.GetTaskByID(s.db, taskID); err == nil && task != nil {
			workDir = strings.TrimSpace(task.WorkDir)
		}

		var inputParts []*types.Part
		parts, saved, saveErr := prepareWechatInboundAttachments(workDir, rt.account.BotID, msg.MessageID, inbound.Attachments)
		if saveErr != nil {
			log.Printf("[ilink] save wechat attachments failed, fallback to inline data: %v", saveErr)
			content = inbound.Text
			inputParts = wechatAttachmentsToInputParts(inbound.Attachments)
		} else {
			content = buildWechatInboundContent(inbound.Text, saved)
			inputParts = parts
		}

		log.Printf("[ilink] received message from %s for task %d: %s (attachments=%d, workDir=%q)",
			msg.FromUserID, taskID, content, len(inbound.Attachments), workDir)

		opts := []task_runner.TaskRuntimeConfigOption{
			task_runner.WithContent(content),
			task_runner.WithWSHub(s.hub),
			task_runner.WithDB(s.db),
			task_runner.WithInputSource(task_runner.TaskInputSourceWeChat),
			task_runner.WithQueueItemID(fmt.Sprintf("wechat-%s-%d", rt.account.BotID, msg.MessageID)),
		}
		if len(inputParts) > 0 {
			opts = append(opts, task_runner.WithInputParts(inputParts))
		}

		rt.startTypingLoop()
		defer rt.stopTypingLoop()

		if err := task_runner.RunTask(taskID, opts...); err != nil {
			log.Printf("[ilink] run task failed: %v", err)
		}
	}
}

// registerTaskListener 注册任务输出监听器，将任务回复转发到微信
func (s *ILinkService) registerTaskListener(rt *ilinkAccount, taskID uint) {
	key := fmt.Sprintf("ilink-%s", rt.account.BotID)
	s.hub.RegisterTaskMessageListener(taskID, key, func(msg *models.TaskMessage) {
		if msg == nil || msg.Role != "assistant" {
			return
		}

		rt.msgMu.Lock()
		toUserID := rt.lastFromUserID
		ctxToken := rt.lastCtxToken
		rt.msgMu.Unlock()
		if toUserID == "" {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		switch msg.Type {
		case task_runner.TaskMessageTypeAssistantAttachment:
			payload, ok := parseAssistantAttachmentPayload(msg.Content)
			if !ok {
				return
			}
			log.Printf("[ilink] forwarding task %d attachment to wechat %s filename=%q", taskID, rt.account.BotID, payload.Filename)
			if err := rt.sendAttachmentToWeChat(ctx, payload); err != nil {
				log.Printf("[ilink] send attachment failed: %v", err)
			}
			return
		case "normalized_entry", "message":
		default:
			return
		}

		content := ""
		switch v := msg.Content.(type) {
		case string:
			content = v
		case []byte:
			content = string(v)
		default:
			content = fmt.Sprintf("%v", v)
		}
		content = strings.TrimSpace(content)
		if content == "" {
			return
		}

		log.Printf("[ilink] forwarding task %d output to wechat %s", taskID, rt.account.BotID)
		if err := rt.bot.SendText(ctx, toUserID, content, ctxToken); err != nil {
			log.Printf("[ilink] send text failed: %v", err)
		}
	})
	rt.listenerID = key
}
