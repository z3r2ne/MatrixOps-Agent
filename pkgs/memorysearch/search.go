package memorysearch

import (
	"context"
	"fmt"
	"strings"

	"matrixops-agent/types"
	database "pkgs/db"
	"pkgs/embedding"

	"gorm.io/gorm"
)

func ArchiveMemoryEntries(ctx context.Context, db *gorm.DB, sessionID string, entries []*types.MemoryEntry) error {
	if db == nil || !embedding.HasEnabledEmbeddingConfig(db) {
		return nil
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || len(entries) == 0 {
		return nil
	}

	pending := filterEntriesPendingSearchArchive(entries)
	if len(pending) == 0 {
		return nil
	}

	content := renderArchivedMemoryTranscript(pending)
	if content == "" {
		return nil
	}

	client, _, err := embedding.GetActiveClient(db)
	if err != nil {
		return err
	}
	vectors, err := client.Embed(ctx, []string{content})
	if err != nil {
		return err
	}

	store, err := ensureStore()
	if err != nil {
		return err
	}
	if err := store.Upsert(ctx, db, indexedDoc{
		ID:          archiveDocID(sessionID, content),
		SessionID:   sessionID,
		SourceKind:  compressedMemorySourceKind,
		Content:     content,
		ContentHash: hashContent(content),
		Embedding:   vectors[0],
	}); err != nil {
		return err
	}

	for _, entry := range pending {
		entry.SearchArchived = true
	}
	return nil
}

func renderArchivedMemoryTranscript(entries []*types.MemoryEntry) string {
	if len(entries) == 0 {
		return ""
	}
	memory := &types.Memory{Entries: entries}
	return strings.TrimSpace(memory.PromptContent())
}

func archiveDocID(sessionID, content string) string {
	sum := hashContent(content)
	if len(sum) >= 16 {
		sum = sum[:16]
	}
	return fmt.Sprintf("archive:%s:%s", sessionID, sum)
}

func ResolveLibraryIDsForSession(db *gorm.DB, sessionID string) ([]uint, error) {
	return database.ResolveMemoryLibraryIDsForSession(db, sessionID)
}

func SearchMulti(ctx context.Context, db *gorm.DB, sessionID string, libraryIDs []uint, queries, keywords []string, topK int) ([]SearchResult, error) {
	queries = normalizeSearchTexts(queries)
	keywords = normalizeSearchTexts(keywords)
	if len(queries) == 0 && len(keywords) == 0 {
		return nil, fmt.Errorf("queries or keywords is required")
	}
	if !embedding.HasEnabledEmbeddingConfig(db) {
		return nil, fmt.Errorf("embedding is not configured")
	}

	var queryVectors [][]float32
	if len(queries) > 0 {
		client, _, err := embedding.GetActiveClient(db)
		if err != nil {
			return nil, err
		}
		queryVectors, err = client.Embed(ctx, queries)
		if err != nil {
			return nil, err
		}
	}

	store, err := ensureStore()
	if err != nil {
		return nil, err
	}
	return store.HybridSearchMulti(ctx, sessionID, libraryIDs, queries, keywords, queryVectors, topK)
}

func FormatSearchResults(results []SearchResult) string {
	if len(results) == 0 {
		return "未找到相关记忆。"
	}
	var builder strings.Builder
	builder.WriteString("记忆检索结果（chromem-go 语义 + bleve 关键词混合）：\n")
	for i, item := range results {
		builder.WriteString(fmt.Sprintf("[%d] score=%.3f kind=%s\n", i+1, item.Score, item.SourceKind))
		if item.SessionID != "" {
			builder.WriteString("session: " + item.SessionID + "\n")
		}
		if item.MemoryLibraryID > 0 {
			builder.WriteString(fmt.Sprintf("memoryLibraryId: %d\n", item.MemoryLibraryID))
		}
		builder.WriteString(item.Content)
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String())
}

func normalizeSearchTexts(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
