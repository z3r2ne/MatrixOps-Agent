package models

import (
	"encoding/json"
	"strings"

	"matrixops-agent/permission"
	"matrixops-agent/types"
)

func MemoryEntryToModel(entry *types.MemoryEntry) *MemoryEntry {
	if entry == nil {
		return nil
	}

	return &MemoryEntry{
		ID:                             entry.ID,
		SessionID:                      entry.SessionID,
		SourceMessageID:                entry.SourceMessageID,
		SourcePartID:                   entry.SourcePartID,
		EntryKind:                      entry.EntryKind,
		Role:                           entry.Role,
		Content:                        entry.Content,
		RawOutput:                      entry.RawOutput,
		Phase:                          entry.Phase,
		ResponsesOutputMessageRaw:      entry.ResponsesOutputMessageRaw,
		ResponsesReasoningItemRawsJSON: entry.ResponsesReasoningItemRawsJSON,
		ReasoningContent:               entry.ReasoningContent,
		ThinkingSignature:              entry.ThinkingSignature,
		CallToolInfo:                   entry.CallToolInfo,
		ToolCallID:                     entry.ToolCallID,
		ToolName:                       entry.ToolName,
		ToolStatus:                     entry.ToolStatus,
		ToolReason:                     entry.ToolReason,
		ToolRequestRawJSON:             entry.ToolRequestRawJSON,
		ToolInputJSON:                  entry.ToolInputJSON,
		ToolOutput:                     entry.ToolOutput,
		ToolSystemMessage:              entry.ToolSystemMessage,
		ToolError:                      entry.ToolError,
		ToolTitle:                      entry.ToolTitle,
		ToolMetadataJSON:               entry.ToolMetadataJSON,
		Synthetic:                      entry.Synthetic,
		SearchArchived:                 entry.SearchArchived,
		CompressionLevel:               entry.CompressionLevel,
		Sequence:                       entry.Sequence,
		TokenCount:                     entry.TokenCount,
		Created:                        entry.Created,
		Updated:                        entry.Updated,
	}
}

func MemoryEntryFromModel(entry *MemoryEntry) *types.MemoryEntry {
	if entry == nil {
		return nil
	}

	return &types.MemoryEntry{
		ID:                             entry.ID,
		SessionID:                      entry.SessionID,
		SourceMessageID:                entry.SourceMessageID,
		SourcePartID:                   entry.SourcePartID,
		EntryKind:                      entry.EntryKind,
		Role:                           entry.Role,
		Content:                        entry.Content,
		RawOutput:                      entry.RawOutput,
		Phase:                          entry.Phase,
		ResponsesOutputMessageRaw:      entry.ResponsesOutputMessageRaw,
		ResponsesReasoningItemRawsJSON: entry.ResponsesReasoningItemRawsJSON,
		ReasoningContent:               entry.ReasoningContent,
		ThinkingSignature:              entry.ThinkingSignature,
		CallToolInfo:                   entry.CallToolInfo,
		ToolCallID:                     entry.ToolCallID,
		ToolName:                       entry.ToolName,
		ToolStatus:                     entry.ToolStatus,
		ToolReason:                     entry.ToolReason,
		ToolRequestRawJSON:             entry.ToolRequestRawJSON,
		ToolInputJSON:                  entry.ToolInputJSON,
		ToolOutput:                     entry.ToolOutput,
		ToolSystemMessage:              entry.ToolSystemMessage,
		ToolError:                      entry.ToolError,
		ToolTitle:                      entry.ToolTitle,
		ToolMetadataJSON:               entry.ToolMetadataJSON,
		Synthetic:                      entry.Synthetic,
		SearchArchived:                 entry.SearchArchived,
		CompressionLevel:               entry.CompressionLevel,
		Sequence:                       entry.Sequence,
		TokenCount:                     entry.TokenCount,
		Created:                        entry.Created,
		Updated:                        entry.Updated,
	}
}

// SessionToModel 将 types.Info 转换为 Session 模型
func SessionToModel(info *types.Info) *Session {
	if info == nil {
		return nil
	}

	s := &Session{
		ID:            info.ID,
		Slug:          info.Slug,
		ProjectID:     info.ProjectID,
		Directory:     info.Directory,
		WorkspaceRoot: info.WorkspaceRoot,
		WorkspacePath: info.WorkspacePath,
		ParentID:      info.ParentID,
		Title:         info.Title,
		Version:       info.Version,
		StartSnapshot: info.StartSnapshot,
		Created:       info.Time.Created,
		Updated:       info.Time.Updated,
		Compacting:    info.Time.Compacting,
		Archived:      info.Time.Archived,
	}

	if len(info.EnabledSkills) > 0 {
		s.EnabledSkills = JSONField{Data: info.EnabledSkills}
	}

	if info.Summary != nil {
		s.Summary = JSONField{Data: info.Summary}
	}
	if info.MemoryAnalysis != nil {
		s.MemoryAnalysis = JSONField{Data: info.MemoryAnalysis}
	}
	if info.CriticalInfo != nil {
		s.CriticalInfo = JSONField{Data: info.CriticalInfo}
	}
	if info.Share != nil {
		s.Share = JSONField{Data: info.Share}
	}
	if info.Permission != nil {
		s.Permission = JSONField{Data: info.Permission}
	}
	if info.Revert != nil {
		s.Revert = JSONField{Data: info.Revert}
	}
	if info.Tokens != nil {
		s.Tokens = JSONField{Data: info.Tokens}
	}

	return s
}

// SessionFromModel 将 Session 模型转换为 types.Info
func SessionFromModel(s *Session) *types.Info {
	if s == nil {
		return nil
	}

	info := &types.Info{
		ID:            s.ID,
		Slug:          s.Slug,
		ProjectID:     s.ProjectID,
		Directory:     s.Directory,
		WorkspaceRoot: s.WorkspaceRoot,
		WorkspacePath: s.WorkspacePath,
		EnabledSkills: nil,
		ParentID:      s.ParentID,
		Title:         s.Title,
		Version:       s.Version,
		StartSnapshot: s.StartSnapshot,
		Time: types.TimeInfo{
			Created:    s.Created,
			Updated:    s.Updated,
			Compacting: s.Compacting,
			Archived:   s.Archived,
		},
	}

	// 转换 Summary
	if s.Summary.Data != nil {
		if summary, ok := s.Summary.Data.(map[string]interface{}); ok {
			info.Summary = &types.Summary{
				Additions: int(getFloat64(summary, "additions")),
				Deletions: int(getFloat64(summary, "deletions")),
				Files:     int(getFloat64(summary, "files")),
			}
		}
	}

	// 转换 Share
	if s.Share.Data != nil {
		if share, ok := s.Share.Data.(map[string]interface{}); ok {
			if url, ok := share["url"].(string); ok {
				info.Share = &types.ShareInfo{URL: url}
			}
		}
	}

	// 转换 Permission
	if s.Permission.Data != nil {
		if perm, ok := s.Permission.Data.(permission.Ruleset); ok {
			info.Permission = perm
		} else {
			// 尝试从 map 转换
			data, _ := json.Marshal(s.Permission.Data)
			var ruleset permission.Ruleset
			json.Unmarshal(data, &ruleset)
			info.Permission = ruleset
		}
	}

	// 转换 Revert
	if s.Revert.Data != nil {
		if revert, ok := s.Revert.Data.(map[string]interface{}); ok {
			info.Revert = &types.RevertInfo{
				MessageID: getString(revert, "messageID"),
				PartID:    getString(revert, "partID"),
				Snapshot:  getString(revert, "snapshot"),
				Diff:      getString(revert, "diff"),
			}
		}
	}

	convertJSONField(s.Tokens.Data, &info.Tokens)
	convertJSONField(s.EnabledSkills.Data, &info.EnabledSkills)
	if s.MemoryAnalysis.Data != nil {
		var analysis types.MemoryAnalysis
		convertJSONField(s.MemoryAnalysis.Data, &analysis)
		info.MemoryAnalysis = &analysis
	}
	if s.CriticalInfo.Data != nil {
		var criticalInfo types.CriticalInfo
		convertJSONField(s.CriticalInfo.Data, &criticalInfo)
		info.CriticalInfo = &criticalInfo
	}

	return info
}

// MessageToModel 将 types.MessageInfo 转换为 Message 模型
func MessageToModel(msg *types.MessageInfo) *Message {
	if msg == nil {
		return nil
	}

	m := &Message{
		ID:            msg.ID,
		SessionID:     msg.SessionID,
		Role:          string(msg.Role),
		MessageKind:   msg.MessageKind,
		MessageOrigin: msg.MessageOrigin,
		ParentID:      msg.ParentID,
		// Mode:       msg.Mode,
		// Agent:      msg.Agent,
		Name:       string(msg.Role),
		Occupation: msg.Occupation,
		ProviderID: msg.ProviderID,
		ModelID:    msg.ModelID,
		System:     msg.System,
		Variant:    msg.Variant,
		Finish:     msg.Finish,
		Cost:       msg.Cost,
		Created:    msg.Time.Created,
		Completed:  msg.Time.Completed,
		State:      msg.State,
		Snapshot:   "",
		Phase:      msg.Phase,
		ResponsesOutputMessageRaw:  msg.ResponsesOutputMessageRaw,
		ResponsesReasoningItemRawsJSON: func() string {
			if len(msg.ResponsesReasoningItemRaws) == 0 {
				return ""
			}
			data, _ := json.Marshal(msg.ResponsesReasoningItemRaws)
			return string(data)
		}(),
		Memory:     JSONField{Data: msg.Memory},
	}

	// if msg.Model != nil {
	// 	m.Model = JSONField{Data: msg.Model}
	// }
	if msg.Tools != nil {
		m.Tools = JSONField{Data: msg.Tools}
	}
	if msg.Summary != nil {
		m.Summary = JSONField{Data: msg.Summary}
	}
	if msg.Tokens != nil {
		m.Tokens = JSONField{Data: msg.Tokens}
	}
	if msg.Error != nil {
		m.Error = JSONField{Data: msg.Error}
	}
	if msg.Path != nil {
		m.Path = JSONField{Data: msg.Path}
	}

	return m
}

// MessageFromModel 将 Message 模型转换为 types.MessageInfo
func MessageFromModel(m *Message) *types.MessageInfo {
	if m == nil {
		return nil
	}

	msg := &types.MessageInfo{
		ID:            m.ID,
		SessionID:     m.SessionID,
		Role:          types.Role(m.Role),
		MessageKind:   m.MessageKind,
		MessageOrigin: m.MessageOrigin,
		ParentID:      m.ParentID,
		// Mode:       m.Mode,
		// Agent:      m.Agent,
		Worker:     m.Name,
		Occupation: m.Occupation,
		ProviderID: m.ProviderID,
		ModelID:    m.ModelID,
		System:     m.System,
		Variant:    m.Variant,
		Finish:     m.Finish,
		Cost:       m.Cost,
		State:      m.State,
		Snapshot:   m.Snapshot,
		Phase:      m.Phase,
		ResponsesOutputMessageRaw: m.ResponsesOutputMessageRaw,
		ResponsesReasoningItemRaws: func() []string {
			if strings.TrimSpace(m.ResponsesReasoningItemRawsJSON) == "" {
				return nil
			}
			var items []string
			if err := json.Unmarshal([]byte(m.ResponsesReasoningItemRawsJSON), &items); err != nil {
				return nil
			}
			return items
		}(),
		Time: types.MessageTime{
			Created:   m.Created,
			Completed: m.Completed,
		},
	}

	// 转换 JSON 字段
	convertJSONField(m.Model.Data, &msg.Model)
	convertJSONField(m.Tokens.Data, &msg.Tokens)
	convertJSONField(m.Error.Data, &msg.Error)
	convertJSONField(m.Path.Data, &msg.Path)
	convertJSONField(m.Memory.Data, &msg.Memory)

	if m.Tools.Data != nil {
		if tools, ok := m.Tools.Data.(map[string]interface{}); ok {
			msg.Tools = make(map[string]bool)
			for k, v := range tools {
				if b, ok := v.(bool); ok {
					msg.Tools[k] = b
				}
			}
		}
	}

	msg.Summary = m.Summary.Data

	return msg
}

// PartToModel 将 types.Part 转换为 Part 模型
func PartToModel(part *types.Part) *Part {
	if part == nil {
		return nil
	}

	p := &Part{
		ID:          part.ID,
		MessageID:   part.MessageID,
		SessionID:   part.SessionID,
		Type:        part.Type,
		Text:        part.Text,
		Reasoning:   part.Reasoning,
		Synthetic:   part.Synthetic,
		Ignored:     part.Ignored,
		Snapshot:    part.Snapshot,
		Hash:        part.Hash,
		Mime:        part.Mime,
		Filename:    part.Filename,
		URL:         part.URL,
		AgentName:   part.AgentName,
		Auto:        part.Auto,
		Description: part.Description,
		Subagent:    part.Subagent,
		Command:     part.Command,
		Attempt:     part.Attempt,
		Reason:      part.Reason,
		Cost:        part.Cost,
	}

	if part.Time != nil {
		p.TimeStart = part.Time.Start
		p.TimeEnd = part.Time.End
		p.TimeCreated = part.Time.Created
		p.TimeCompacted = part.Time.Compacted
	}

	if part.Tool != nil {
		p.Tool = JSONField{Data: part.Tool}
	}
	if part.Metadata != nil {
		p.Metadata = JSONField{Data: part.Metadata}
	}
	if part.Files != nil {
		p.Files = JSONField{Data: part.Files}
	}
	if part.Source != nil {
		p.Source = JSONField{Data: part.Source}
	}
	if part.Model != nil {
		p.Model = JSONField{Data: part.Model}
	}
	if part.Error != nil {
		p.Error = JSONField{Data: part.Error}
	}
	if part.Tokens != nil {
		p.Tokens = JSONField{Data: part.Tokens}
	}
	return p
}

// PartFromModel 将 Part 模型转换为 types.Part
func PartFromModel(p *Part) *types.Part {
	if p == nil {
		return nil
	}

	part := &types.Part{
		ID:          p.ID,
		MessageID:   p.MessageID,
		SessionID:   p.SessionID,
		Type:        p.Type,
		Text:        p.Text,
		Reasoning:   p.Reasoning,
		Synthetic:   p.Synthetic,
		Ignored:     p.Ignored,
		Snapshot:    p.Snapshot,
		Hash:        p.Hash,
		Mime:        p.Mime,
		Filename:    p.Filename,
		URL:         p.URL,
		AgentName:   p.AgentName,
		Auto:        p.Auto,
		Description: p.Description,
		Subagent:    p.Subagent,
		Command:     p.Command,
		Attempt:     p.Attempt,
		Reason:      p.Reason,
		Cost:        p.Cost,
	}

	if p.TimeStart != 0 || p.TimeEnd != 0 || p.TimeCreated != 0 || p.TimeCompacted != 0 {
		part.Time = &types.PartTime{
			Start:     p.TimeStart,
			End:       p.TimeEnd,
			Created:   p.TimeCreated,
			Compacted: p.TimeCompacted,
		}
	}

	// 转换 JSON 字段
	convertJSONField(p.Tool.Data, &part.Tool)
	convertJSONField(p.Model.Data, &part.Model)
	convertJSONField(p.Error.Data, &part.Error)
	convertJSONField(p.Tokens.Data, &part.Tokens)

	if p.Metadata.Data != nil {
		if metadata, ok := p.Metadata.Data.(map[string]interface{}); ok {
			part.Metadata = metadata
		}
	}

	if p.Files.Data != nil {
		if files, ok := p.Files.Data.([]interface{}); ok {
			part.Files = make([]string, 0, len(files))
			for _, f := range files {
				if s, ok := f.(string); ok {
					part.Files = append(part.Files, s)
				}
			}
		}
	}

	part.Source = p.Source.Data

	return part
}

// 辅助函数
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getFloat64(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func convertJSONField(src interface{}, dst interface{}) {
	if src == nil {
		return
	}
	data, _ := json.Marshal(src)
	json.Unmarshal(data, dst)
}
