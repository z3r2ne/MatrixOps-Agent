package session

import (
	sessionmemory "matrixops-agent/session/memory"
	"matrixops-agent/types"

	"gorm.io/gorm"
)

func buildMemoryEntriesFromMessage(message *WithParts) ([]*types.MemoryEntry, error) {
	return sessionmemory.BuildEntriesFromMessage((*types.WithParts)(message))
}

func syncPartMemory(db *gorm.DB, message *MessageInfo, part *Part) error {
	return sessionmemory.SyncPartMemory(db, (*types.MessageInfo)(message), (*types.Part)(part))
}

func memoryEntriesToChatHistory(entries []*types.MemoryEntry) []*types.ChatHistoryItem {
	return sessionmemory.MemoryEntriesToChatHistory(entries)
}

func totalMemoryTokens(entries []*types.MemoryEntry) int {
	return sessionmemory.TotalMemoryTokens(entries)
}

func estimateMemoryEntryTokenCount(entry *types.MemoryEntry) int {
	return sessionmemory.EstimateMemoryEntryTokenCount(entry)
}
