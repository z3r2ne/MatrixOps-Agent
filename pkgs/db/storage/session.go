package storage

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"matrixops-agent/permission"
	"matrixops-agent/snapshot"
	"matrixops-agent/types"
	"matrixops-agent/util"
	database "pkgs/db"
	"pkgs/db/models"
)

// NewSessionWithMemoryLibraries 创建新会话，并按给定记忆库 ID 注入初始对话。
// libraryIDs 为 nil 时从项目配置解析；为空切片时不注入。
func NewSessionWithMemoryLibraries(db *gorm.DB, projectID string, directory string, libraryIDs []uint) (*types.Info, error) {
	startSnapshot, err := snapshot.Track(projectID, directory)
	if err != nil {
		return nil, err
	}

	session := &types.Info{
		ID:            util.Descending("session"),
		Slug:          util.Slug(),
		ProjectID:     projectID,
		Directory:     directory,
		Version:       util.Version(),
		StartSnapshot: startSnapshot,
	}
	if err := ensureSessionWorkspaceInfo(db, session); err != nil {
		return nil, err
	}
	sessionModel := models.SessionToModel(session)
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(sessionModel).Error; err != nil {
			return err
		}
		return seedSessionMemoryLibraries(tx, session, libraryIDs)
	}); err != nil {
		return nil, err
	}
	return models.SessionFromModel(sessionModel), nil
}

// NewSession 创建新会话
func NewSession(db *gorm.DB, projectID string, directory string) (*types.Info, error) {
	return NewSessionWithMemoryLibraries(db, projectID, directory, nil)
}

// hasSession 检查会话是否存在
func hasSession(db *gorm.DB, sessionID string) (bool, error) {
	var session models.Session
	if err := db.Where("id = ?", sessionID).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
func CreateOrGetSession(db *gorm.DB, projectID string, directory string, sessionID string) (*types.Info, error) {
	has, err := hasSession(db, sessionID)
	if err != nil {
		return nil, err
	}
	if has {
		return GetSession(db, sessionID)
	}
	return NewSession(db, projectID, directory)
}

// GetSession 获取会话信息
func GetSession(db *gorm.DB, sessionID string) (*types.Info, error) {
	// inst := project.Current()
	// if inst == nil {
	// 	return nil, project.ErrNoInstance
	// }

	if err := ensureSessionDataSchema(db); err != nil {
		return nil, err
	}

	var session models.Session
	if err := db.Where("id = ?", sessionID).First(&session).Error; err != nil {
		return nil, err
	}
	info := models.SessionFromModel(&session)
	return info, nil
}

// UpdateSession 更新会话信息
func UpdateSession(db *gorm.DB, newSession *types.Info) error {
	if newSession == nil {
		return nil
	}

	if err := ensureSessionDataSchema(db); err != nil {
		return err
	}

	newSession.Time.Updated = time.Now().UnixMilli()
	session := models.SessionToModel(newSession)

	return db.Save(session).Error
}

// UpdateSessionByCallback 通过回调函数更新会话
func UpdateSessionByCallback(db *gorm.DB, sessionID string, editor func(*types.Info) error) (*types.Info, error) {
	if err := ensureSessionDataSchema(db); err != nil {
		return nil, err
	}

	var info *types.Info

	err := db.Transaction(func(tx *gorm.DB) error {
		var session models.Session
		if err := tx.Where("id = ? ", sessionID).First(&session).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return NotFoundError{Path: sessionID}
			}
			return err
		}

		info = models.SessionFromModel(&session)
		info.Time.Updated = time.Now().UnixMilli()

		if err := editor(info); err != nil {
			return err
		}

		updatedSession := models.SessionToModel(info)
		return tx.Save(updatedSession).Error
	})

	if err != nil {
		return nil, err
	}

	return info, nil
}

func GetPartsByMessageID(db *gorm.DB, messageID string) ([]*types.Part, error) {
	if err := ensureSessionDataSchema(db); err != nil {
		return nil, err
	}
	var partModels []*models.Part
	if err := db.Where("message_id = ?", messageID).Order("time_created ASC, id ASC").Find(&partModels).Error; err != nil {
		return nil, err
	}

	parts := []*types.Part{}
	for _, partModel := range partModels {
		parts = append(parts, models.PartFromModel(partModel))
	}
	return parts, nil
}
func GetSessionMessageParts(db *gorm.DB, sessionID string, workers ...string) ([]*types.WithParts, error) {
	return GetSessionMessagePartsWithMe(db, sessionID, "", workers...)
}

// GetSessionMessageParts 获取会话的所有消息及其部件
func GetSessionMessagePartsWithMe(db *gorm.DB, sessionID string, me string, workers ...string) ([]*types.WithParts, error) {
	if err := ensureSessionDataSchema(db); err != nil {
		return nil, err
	}

	// 查询所有消息
	var messageModels []models.Message
	if err := db.Where("session_id = ?", sessionID).Find(&messageModels).Error; err != nil {
		return nil, err
	}

	messages := []*types.WithParts{}
	workerSet := map[string]struct{}{}
	if len(workers) > 0 {
		for _, worker := range workers {
			if strings.TrimSpace(worker) == "" {
				continue
			}
			workerSet[worker] = struct{}{}
		}
	}

	for _, msgModel := range messageModels {
		msg := models.MessageFromModel(&msgModel)

		// 查询消息的所有部件
		var partModels []models.Part
		if err := db.Where("message_id = ?", msgModel.ID).Order("time_created ASC, id ASC").Find(&partModels).Error; err != nil {
			return nil, err
		}

		parts := []*types.Part{}
		for _, partModel := range partModels {
			part := models.PartFromModel(&partModel)
			parts = append(parts, part)
		}

		// 按 ID 排序部件
		sort.Slice(parts, func(i, j int) bool {
			return parts[i].ID < parts[j].ID
		})

		if len(workerSet) > 0 {
			name := ""
			worker := ""
			if msg != nil {
				name = string(msg.Role)
				worker = msg.Worker
			}
			_, matchName := workerSet[name]
			_, matchWorker := workerSet[worker]
			if !matchName && !matchWorker {
				continue
			}
		}
		messages = append(messages, &types.WithParts{Info: msg, Parts: parts})
	}

	// 按消息 ID 排序
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Info.ID < messages[j].Info.ID
	})

	// if me != "" {
	// 	copyMsg := func(msg *types.WithParts) *types.WithParts {
	// 		copy := *msg
	// 		info := *copy.Info
	// 		copy.Info = &info
	// 		return &copy
	// 	}
	// 	newMessages := []*types.WithParts{}
	// 	for _, msg := range messages {
	// 		if msg.Info.Name == me {
	// 			newMsg := copyMsg(msg)
	// 			newMsg.Info.Role = types.RoleUser
	// 			newMessages = append(newMessages, newMsg)
	// 		} else {
	// 			newMsg := copyMsg(msg)
	// 			newMsg.Info.Role = types.RoleAssistant
	// 			newMessages = append(newMessages, newMsg)
	// 		}
	// 	}
	// 	messages = newMessages
	// }

	return messages, nil
}

// UpdateMessageInfo 更新消息信息
func UpdateMessageInfo(db *gorm.DB, message *types.MessageInfo) error {
	if message == nil {
		return nil
	}

	if err := ensureSessionDataSchema(db); err != nil {
		return err
	}

	messageModel := models.MessageToModel(message)
	return db.Save(messageModel).Error
}

// UpdateSessionTitle 更新会话标题
func UpdateSessionTitle(db *gorm.DB, sessionID string, title string) error {
	_, err := UpdateSessionByCallback(db, sessionID, func(draft *types.Info) error {
		draft.Title = title
		return nil
	})
	return err
}

func UpdateSessionTokens(db *gorm.DB, sessionID string, tokens *types.MessageTokens) error {
	_, err := UpdateSessionByCallback(db, sessionID, func(draft *types.Info) error {
		draft.Tokens = tokens
		return nil
	})
	return err
}

// CreateSession 创建新会话
func CreateSession(db *gorm.DB, projectID string, directory string, title string, parentID string, ruleset permission.Ruleset) (*types.Info, error) {
	// 在创建 Session 时立即创建初始快照
	startSnapshot, err := snapshot.Track(projectID, directory)
	if err != nil {
		return nil, err
	}

	if err := ensureSessionDataSchema(db); err != nil {
		return nil, err
	}

	info := &types.Info{
		ID:            util.Descending("session"),
		Slug:          util.Slug(),
		ProjectID:     projectID,
		Directory:     directory,
		ParentID:      parentID,
		Title:         title,
		Version:       util.Version(),
		Permission:    ruleset,
		StartSnapshot: startSnapshot,
		Time: types.TimeInfo{
			Created: time.Now().UnixMilli(),
			Updated: time.Now().UnixMilli(),
		},
	}
	if err := ensureSessionWorkspaceInfo(db, info); err != nil {
		return nil, err
	}

	// 如果没有标题，设置默认标题
	if info.Title == "" {
		if parentID != "" {
			info.Title = "Child session - " + time.Now().Format("15:04:05")
		} else {
			info.Title = "New session - " + time.Now().Format("15:04:05")
		}
	}

	session := models.SessionToModel(info)
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(session).Error; err != nil {
			return err
		}
		return seedSessionMemoryLibraries(tx, info, nil)
	}); err != nil {
		return nil, err
	}

	return info, nil
}

func seedSessionMemoryLibraries(db *gorm.DB, session *types.Info, libraryIDs []uint) error {
	if db == nil || session == nil {
		return nil
	}
	if libraryIDs == nil {
		projectID := strings.TrimSpace(session.ProjectID)
		if projectID == "" {
			return nil
		}
		parsed, err := strconv.ParseUint(projectID, 10, 64)
		if err != nil || parsed == 0 {
			return nil
		}
		project, err := database.GetProjectByID(db, uint(parsed))
		if err != nil || project == nil {
			return err
		}
		libraryIDs = project.MemoryLibraryIDs.Slice()
	}
	if len(libraryIDs) == 0 {
		return nil
	}
	projectID := strings.TrimSpace(session.ProjectID)
	if projectID == "" {
		return nil
	}
	libraries, err := database.GetMemoryLibrariesByIDs(db, libraryIDs)
	if err != nil || len(libraries) == 0 {
		return err
	}
	libraryByID := make(map[uint]models.MemoryLibrary, len(libraries))
	for _, library := range libraries {
		libraryByID[library.ID] = library
	}

	baseTime := time.Now().UnixMilli()
	for index, libraryID := range libraryIDs {
		library, ok := libraryByID[libraryID]
		if !ok {
			continue
		}
		if library.IsRag || library.IsTemporary {
			continue
		}
		content := strings.TrimSpace(library.Content)
		if content == "" {
			continue
		}
		userCreated := baseTime + int64(index*2)
		assistantCreated := userCreated + 1

		userMessage := &types.MessageInfo{
			ID:        util.Ascending("message"),
			SessionID: session.ID,
			Role:      types.RoleUser,
			Time:      types.MessageTime{Created: userCreated},
			State:     "completed",
		}
		if err := UpdateMessageInfo(db, userMessage); err != nil {
			return err
		}
		userPart := &types.Part{
			ID:        util.Ascending("part"),
			MessageID: userMessage.ID,
			SessionID: session.ID,
			Type:      types.PartTypeText,
			Text:      "总结一下。",
			Synthetic: true,
			Time:      &types.PartTime{Start: userCreated, End: userCreated, Created: userCreated},
		}
		if _, err := UpdatePart(db, userPart); err != nil {
			return err
		}
		if err := CreateMemoryEntry(db, &types.MemoryEntry{
			SessionID:       session.ID,
			SourceMessageID: userMessage.ID,
			SourcePartID:    userPart.ID,
			EntryKind:       "history",
			Role:            "user",
			Content:         userPart.Text,
			Synthetic:       true,
			Created:         userCreated,
			Updated:         userCreated,
		}); err != nil {
			return err
		}

		assistantMessage := &types.MessageInfo{
			ID:        util.Ascending("message"),
			SessionID: session.ID,
			Role:      types.RoleAssistant,
			Time:      types.MessageTime{Created: assistantCreated},
			State:     "completed",
		}
		if err := UpdateMessageInfo(db, assistantMessage); err != nil {
			return err
		}
		assistantPart := &types.Part{
			ID:        util.Ascending("part"),
			MessageID: assistantMessage.ID,
			SessionID: session.ID,
			Type:      types.PartTypeText,
			Text:      content,
			Synthetic: true,
			Time:      &types.PartTime{Start: assistantCreated, End: assistantCreated, Created: assistantCreated},
		}
		if _, err := UpdatePart(db, assistantPart); err != nil {
			return err
		}
		if err := CreateMemoryEntry(db, &types.MemoryEntry{
			SessionID:       session.ID,
			SourceMessageID: assistantMessage.ID,
			SourcePartID:    assistantPart.ID,
			EntryKind:       "history",
			Role:            "assistant",
			Content:         content,
			Synthetic:       true,
			Created:         assistantCreated,
			Updated:         assistantCreated,
		}); err != nil {
			return err
		}
	}
	return nil
}

func ensureSessionWorkspaceInfo(db *gorm.DB, info *types.Info) error {
	if info == nil {
		return nil
	}
	if strings.TrimSpace(info.WorkspacePath) != "" {
		if strings.TrimSpace(info.WorkspaceRoot) == "" {
			info.WorkspaceRoot = filepath.Dir(info.WorkspacePath)
		}
		return os.MkdirAll(info.WorkspacePath, 0755)
	}
	if strings.TrimSpace(info.WorkspaceRoot) != "" {
		info.WorkspacePath = filepath.Join(info.WorkspaceRoot, "workspace")
		return os.MkdirAll(info.WorkspacePath, 0755)
	}

	projectName := resolveSessionWorkspaceProjectName(db, info.ProjectID, info.Directory)
	root, workspace, err := database.NewSessionWorkspace(projectName, info.Directory)
	if err != nil {
		return err
	}
	info.WorkspaceRoot = root
	info.WorkspacePath = workspace
	return nil
}

func resolveSessionWorkspaceProjectName(db *gorm.DB, projectID string, directory string) string {
	projectID = strings.TrimSpace(projectID)
	if db != nil && projectID != "" {
		if parsed, err := strconv.ParseUint(projectID, 10, 64); err == nil && parsed > 0 {
			if project, err := database.GetProjectByID(db, uint(parsed)); err == nil && strings.TrimSpace(project.Name) != "" {
				return strings.TrimSpace(project.Name)
			}
		}
	}
	name := filepath.Base(filepath.Clean(strings.TrimSpace(directory)))
	name = strings.TrimSpace(name)
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

func UpdatePart(db *gorm.DB, part *types.Part) (*types.Part, error) {
	if part == nil {
		return nil, nil
	}
	if err := ensureSessionDataSchema(db); err != nil {
		return nil, err
	}
	partModel := models.PartToModel(part)
	if err := db.Save(partModel).Error; err != nil {
		return nil, err
	}
	return part, nil
}

func DeletePart(db *gorm.DB, part *types.Part) error {
	if part == nil {
		return nil
	}
	partModel := models.PartToModel(part)
	if err := db.Delete(partModel).Error; err != nil {
		return err
	}
	return nil
}

func DeleteMessageBySession(db *gorm.DB, sessionID string) error {
	return db.Delete(&models.Message{}, "session_id = ?", sessionID).Error
}

func DeleteMessage(db *gorm.DB, message *types.MessageInfo) error {
	if message == nil {
		return nil
	}
	messageModel := models.MessageToModel(message)
	if err := db.Delete(messageModel).Error; err != nil {
		return err
	}
	return nil
}

func DeleteSession(db *gorm.DB, sessionID string) error {
	return db.Delete(&models.Session{}, "id = ?", sessionID).Error
}

func DeleteParts(db *gorm.DB, messageID string) error {
	return db.Delete(&models.Part{}, "message_id = ?", messageID).Error
}

func InitStorage(db *gorm.DB) error {
	return ensureSessionDataSchema(db)
}
