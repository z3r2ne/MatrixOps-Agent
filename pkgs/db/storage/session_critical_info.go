package storage

import (
	"fmt"
	"strings"

	"matrixops-agent/types"

	"gorm.io/gorm"
)

// GetSessionCriticalInfo 读取会话关键信息。
func GetSessionCriticalInfo(db *gorm.DB, sessionID string) (*types.CriticalInfo, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session id is required")
	}
	info, err := GetSession(db, sessionID)
	if err != nil {
		return nil, err
	}
	if info == nil || info.CriticalInfo == nil {
		return &types.CriticalInfo{}, nil
	}
	return cloneCriticalInfo(info.CriticalInfo), nil
}

// UpdateSessionCriticalInfo 更新会话关键信息。
func UpdateSessionCriticalInfo(db *gorm.DB, sessionID string, criticalInfo *types.CriticalInfo) (*types.Info, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session id is required")
	}
	return UpdateSessionByCallback(db, sessionID, func(info *types.Info) error {
		if criticalInfo == nil || len(criticalInfo.Items) == 0 {
			info.CriticalInfo = nil
			return nil
		}
		info.CriticalInfo = cloneCriticalInfo(criticalInfo)
		return nil
	})
}

func cloneCriticalInfo(src *types.CriticalInfo) *types.CriticalInfo {
	if src == nil {
		return nil
	}
	out := &types.CriticalInfo{
		Items: make([]types.CriticalInfoItem, 0, len(src.Items)),
	}
	for _, item := range src.Items {
		clone := item
		if len(item.MatchSources) > 0 {
			clone.MatchSources = append([]types.CriticalInfoMatchSource(nil), item.MatchSources...)
		}
		if item.AsyncTask != nil {
			taskCopy := *item.AsyncTask
			if item.AsyncTask.Params != nil {
				taskCopy.Params = make(map[string]interface{}, len(item.AsyncTask.Params))
				for key, value := range item.AsyncTask.Params {
					taskCopy.Params[key] = value
				}
			}
			clone.AsyncTask = &taskCopy
		}
		out.Items = append(out.Items, clone)
	}
	return out
}
