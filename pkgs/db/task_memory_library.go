package database

import (
	"fmt"
	"strings"

	"pkgs/db/models"

	"gorm.io/gorm"
)

func GetTaskBySessionID(db *gorm.DB, sessionID string) (*models.Task, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var task models.Task
	err := db.Where("session_id = ?", sessionID).Order("id DESC").First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func ListMemoryLibraries(db *gorm.DB, includeTemporary bool, isRag bool) ([]models.MemoryLibrary, error) {
	query := db.Where("is_rag = ?", isRag).Order("updated_at DESC, id DESC")
	if !includeTemporary {
		query = query.Where("is_temporary = ?", false)
	}
	var libraries []models.MemoryLibrary
	err := query.Find(&libraries).Error
	return libraries, err
}

func PromoteMemoryLibrary(db *gorm.DB, libraryID uint, name string) (*models.MemoryLibrary, error) {
	library, err := GetMemoryLibraryByID(db, libraryID)
	if err != nil {
		return nil, err
	}
	if !library.IsTemporary {
		return library, nil
	}
	if !library.IsRag {
		return nil, fmt.Errorf("只能转正 RAG 知识库")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = strings.TrimSpace(library.Name)
	}
	if name == "" {
		return nil, fmt.Errorf("memory library name is required")
	}
	library.Name = name
	library.IsTemporary = false
	library.TaskID = nil
	if err := UpdateMemoryLibrary(db, library); err != nil {
		return nil, err
	}
	return library, nil
}

func normalizePermanentMemoryLibraryIDs(db *gorm.DB, ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("至少选择一个 RAG 知识库")
	}
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		library, err := GetMemoryLibraryByID(db, id)
		if err != nil {
			return nil, fmt.Errorf("RAG 知识库 %d 不存在", id)
		}
		if !library.IsRag {
			return nil, fmt.Errorf("不能选择非 RAG 知识库")
		}
		if library.IsTemporary {
			return nil, fmt.Errorf("不能选择临时 RAG 知识库")
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("至少选择一个 RAG 知识库")
	}
	return out, nil
}

func ApplyTaskMemoryLibraryAfterCreate(db *gorm.DB, task *models.Task) error {
	if task == nil || task.ID == 0 {
		return nil
	}
	mode := models.NormalizeTaskMemoryLibraryMode(task.MemoryLibraryMode)
	task.MemoryLibraryMode = mode

	switch mode {
	case models.TaskMemoryLibraryModeNone:
		task.MemoryLibraryIDs = nil
	case models.TaskMemoryLibraryModeLibraries:
		ids, err := normalizePermanentMemoryLibraryIDs(db, []uint(task.MemoryLibraryIDs))
		if err != nil {
			return err
		}
		task.MemoryLibraryIDs = models.UintSlice(ids)
	case models.TaskMemoryLibraryModeTemporary:
		library := &models.MemoryLibrary{
			Name:        fmt.Sprintf("临时 RAG · 任务 %d", task.ID),
			IsRag:       true,
			IsTemporary: true,
			TaskID:      &task.ID,
		}
		if err := CreateMemoryLibrary(db, library); err != nil {
			return err
		}
		task.MemoryLibraryIDs = models.UintSlice{library.ID}
	default:
		task.MemoryLibraryIDs = nil
	}

	return db.Model(&models.Task{}).Where("id = ?", task.ID).
		Select("MemoryLibraryMode", "MemoryLibraryIDs").
		Updates(models.Task{
			MemoryLibraryMode: task.MemoryLibraryMode,
			MemoryLibraryIDs:  task.MemoryLibraryIDs,
		}).Error
}

func ResolveMemoryLibraryIDsForTask(task *models.Task, project *models.Project) []uint {
	if task != nil && strings.TrimSpace(task.MemoryLibraryMode) != "" {
		switch models.NormalizeTaskMemoryLibraryMode(task.MemoryLibraryMode) {
		case models.TaskMemoryLibraryModeNone:
			return nil
		case models.TaskMemoryLibraryModeTemporary, models.TaskMemoryLibraryModeLibraries:
			return append([]uint(nil), task.MemoryLibraryIDs.Slice()...)
		}
	}
	if project != nil {
		return append([]uint(nil), project.MemoryLibraryIDs.Slice()...)
	}
	return nil
}

func ResolveMemoryLibraryIDsForSession(db *gorm.DB, sessionID string) ([]uint, error) {
	sessionID = strings.TrimSpace(sessionID)
	if db == nil || sessionID == "" {
		return nil, nil
	}
	task, taskErr := GetTaskBySessionID(db, sessionID)
	if taskErr == nil && task != nil && strings.TrimSpace(task.MemoryLibraryMode) != "" {
		return ResolveMemoryLibraryIDsForTask(task, nil), nil
	}
	return nil, nil
}

type sessionProjectRef struct {
	projectID uint
}

func getSessionProjectRef(db *gorm.DB, sessionID string) (*sessionProjectRef, error) {
	var session models.Session
	if err := db.Select("project_id").Where("id = ?", sessionID).First(&session).Error; err != nil {
		return nil, err
	}
	parsed, err := parseUintString(session.ProjectID)
	if err != nil || parsed == 0 {
		return &sessionProjectRef{}, nil
	}
	return &sessionProjectRef{projectID: parsed}, nil
}

func parseUintString(value string) (uint, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	var parsed uint64
	_, err := fmt.Sscanf(value, "%d", &parsed)
	if err != nil {
		return 0, err
	}
	return uint(parsed), nil
}

func PopulateTaskCreateMemoryLibrary(task *models.Task, req models.TaskCreate) {
	if task == nil {
		return
	}
	task.MemoryLibraryMode = models.NormalizeTaskMemoryLibraryMode(req.MemoryLibraryMode)
	task.MemoryLibraryIDs = models.UintSlice(append([]uint(nil), req.MemoryLibraryIDs...))
}
