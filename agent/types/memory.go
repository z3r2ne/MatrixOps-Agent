package types

import agentmemory "matrixops.local/memory"

type Memory = agentmemory.Memory

type MemoryEntry = agentmemory.MemoryEntry

type ChatHistoryItem = agentmemory.ChatHistoryItem

type ChatHistoryContentPart = agentmemory.ChatHistoryContentPart

type ChatHistoryImageURL = agentmemory.ChatHistoryImageURL

type ChatHistoryNativeToolCall = agentmemory.ChatHistoryNativeToolCall

type LatestToolCall = agentmemory.LatestToolCall

type FilePrompt = agentmemory.FilePrompt

func MemoryEntryByteSize(entry *MemoryEntry) int {
	return agentmemory.MemoryEntryByteSize(entry)
}

func CloneChatHistoryContentParts(parts []ChatHistoryContentPart) []ChatHistoryContentPart {
	return agentmemory.CloneChatHistoryContentParts(parts)
}

func formatMemoryTimestamp(created int64) string {
	return agentmemory.FormatMemoryTimestamp(created)
}

func SerializeMemoryEntriesJSON(entries []*MemoryEntry) string {
	return agentmemory.SerializeMemoryEntriesJSON(entries)
}
