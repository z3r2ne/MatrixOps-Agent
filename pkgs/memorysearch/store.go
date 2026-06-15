package memorysearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	chromem "github.com/philippgille/chromem-go"
	"gorm.io/gorm"
)

type Store struct {
	mu         sync.RWMutex
	chromemDB  *chromem.DB
	collection *chromem.Collection
	bleveIndex bleve.Index
}

type indexedDoc struct {
	ID              string
	SessionID       string
	MemoryEntryID   uint
	MemoryLibraryID uint
	SourceKind      string
	Content         string
	ContentHash     string
	Embedding       []float32
}

func openStore() (*Store, error) {
	dataDir, err := database.DataDir()
	if err != nil {
		return nil, err
	}
	chromemPath := filepath.Join(dataDir, "memory-chromem")
	blevePath := filepath.Join(dataDir, "memory-bleve.bleve")

	chromemDB, err := chromem.NewPersistentDB(chromemPath, true)
	if err != nil {
		return nil, fmt.Errorf("open chromem store: %w", err)
	}

	noopEmbed := func(context.Context, string) ([]float32, error) {
		return nil, errors.New("embeddings are managed externally")
	}
	collection, err := chromemDB.GetOrCreateCollection(collectionName, nil, noopEmbed)
	if err != nil {
		return nil, fmt.Errorf("open chromem collection: %w", err)
	}

	bleveIndex, err := openBleveIndex(blevePath)
	if err != nil {
		return nil, fmt.Errorf("open bleve index: %w", err)
	}

	return &Store{
		chromemDB:  chromemDB,
		collection: collection,
		bleveIndex: bleveIndex,
	}, nil
}

func openBleveIndex(path string) (bleve.Index, error) {
	if _, err := os.Stat(path); err == nil {
		return bleve.Open(path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	mapping := bleve.NewIndexMapping()
	docMapping := bleve.NewDocumentMapping()
	docMapping.AddFieldMappingsAt("content", bleve.NewTextFieldMapping())
	docMapping.AddFieldMappingsAt("sessionId", bleve.NewKeywordFieldMapping())
	docMapping.AddFieldMappingsAt("sourceKind", bleve.NewKeywordFieldMapping())
	docMapping.AddFieldMappingsAt("memoryEntryId", bleve.NewNumericFieldMapping())
	docMapping.AddFieldMappingsAt("memoryLibraryId", bleve.NewNumericFieldMapping())
	mapping.AddDocumentMapping("_default", docMapping)

	return bleve.New(path, mapping)
}

func (s *Store) Upsert(ctx context.Context, db *gorm.DB, doc indexedDoc) error {
	if s == nil || doc.ID == "" || strings.TrimSpace(doc.Content) == "" {
		return nil
	}
	if len(doc.Embedding) == 0 {
		return fmt.Errorf("embedding is required for doc %s", doc.ID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.collection.AddDocument(ctx, chromem.Document{
		ID:        doc.ID,
		Content:   doc.Content,
		Embedding: doc.Embedding,
		Metadata:  chromemMetadata(doc),
	}); err != nil {
		return fmt.Errorf("chromem upsert: %w", err)
	}

	if err := s.bleveIndex.Index(doc.ID, bleveDoc(doc)); err != nil {
		return fmt.Errorf("bleve upsert: %w", err)
	}

	chunk := &models.MemorySearchDocument{
		SessionID:       doc.SessionID,
		MemoryEntryID:   doc.MemoryEntryID,
		MemoryLibraryID: doc.MemoryLibraryID,
		SourceKind:      doc.SourceKind,
		Content:         doc.Content,
		ContentHash:     doc.ContentHash,
		Dimension:       len(doc.Embedding),
	}
	return storage.UpsertMemorySearchDocument(db, chunk)
}

func (s *Store) DeleteMemoryLibraryDocument(ctx context.Context, db *gorm.DB, libraryID uint) error {
	if s == nil || libraryID == 0 {
		return nil
	}
	docID := memoryLibraryDocID(libraryID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.deleteDoc(ctx, docID); err != nil {
		return err
	}
	return storage.DeleteMemorySearchDocumentsByLibrary(db, libraryID)
}

func (s *Store) deleteDoc(ctx context.Context, docID string) error {
	if err := s.collection.Delete(ctx, nil, nil, docID); err != nil {
		return fmt.Errorf("chromem delete: %w", err)
	}
	if err := s.bleveIndex.Delete(docID); err != nil {
		return fmt.Errorf("bleve delete: %w", err)
	}
	return nil
}

func (s *Store) HybridSearchMulti(ctx context.Context, sessionID string, libraryIDs []uint, queries, keywords []string, queryVectors [][]float32, topK int) ([]SearchResult, error) {
	if s == nil {
		return nil, fmt.Errorf("memory search store is not initialized")
	}
	if topK <= 0 {
		topK = models.DefaultMemorySearchTopK
	}
	if sessionID == "" && len(libraryIDs) == 0 {
		return nil, fmt.Errorf("sessionID or libraryIDs is required")
	}
	if len(queries) == 0 && len(keywords) == 0 {
		return nil, fmt.Errorf("queries or keywords is required")
	}

	candidateK := topK * 5
	if candidateK < topK {
		candidateK = topK
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	lists := make([][]rankedHit, 0, len(queries)+len(keywords))
	for i := range queries {
		if i >= len(queryVectors) {
			break
		}
		hits, err := s.semanticSearch(ctx, sessionID, libraryIDs, queryVectors[i], candidateK)
		if err != nil {
			return nil, err
		}
		if len(hits) > 0 {
			lists = append(lists, hits)
		}
	}
	for _, keyword := range keywords {
		hits, err := s.keywordSearch(sessionID, libraryIDs, keyword, candidateK)
		if err != nil {
			return nil, err
		}
		if len(hits) > 0 {
			lists = append(lists, hits)
		}
	}

	fused := reciprocalRankFusionLists(lists, topK)
	if len(fused) == 0 {
		return nil, nil
	}

	out := make([]SearchResult, 0, len(fused))
	for _, hit := range fused {
		doc, err := s.collection.GetByID(ctx, hit.docID)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(doc.Content)
		if len(content) > 600 {
			content = content[:600] + "..."
		}
		out = append(out, SearchResult{
			MemoryEntryID:   parseUintMeta(doc.Metadata["memoryEntryId"]),
			MemoryLibraryID: parseUintMeta(doc.Metadata["memoryLibraryId"]),
			SessionID:       doc.Metadata["sessionId"],
			SourceKind:      doc.Metadata["sourceKind"],
			Score:           hit.score,
			Content:         content,
		})
	}
	return out, nil
}

func (s *Store) semanticSearch(ctx context.Context, sessionID string, libraryIDs []uint, queryVector []float32, topK int) ([]rankedHit, error) {
	if len(queryVector) == 0 {
		return nil, nil
	}
	total := s.collection.Count()
	if total <= 0 {
		return nil, nil
	}
	nResults := topK * 10
	if nResults < topK {
		nResults = topK
	}
	if nResults > total {
		nResults = total
	}

	results, err := s.collection.QueryEmbedding(ctx, queryVector, nResults, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("chromem query: %w", err)
	}

	hits := make([]rankedHit, 0, len(results))
	for _, result := range results {
		if !matchesScope(sessionID, libraryIDs, result.Metadata) {
			continue
		}
		hits = append(hits, rankedHit{
			docID: result.ID,
			score: float64(result.Similarity),
		})
		if len(hits) >= topK {
			break
		}
	}
	return hits, nil
}

func (s *Store) keywordSearch(sessionID string, libraryIDs []uint, queryText string, topK int) ([]rankedHit, error) {
	boolQuery := bleve.NewBooleanQuery()
	textQuery := bleve.NewMatchQuery(queryText)
	textQuery.SetField("content")
	boolQuery.AddMust(textQuery)

	scopeQuery := buildScopeQuery(sessionID, libraryIDs)
	if scopeQuery != nil {
		boolQuery.AddMust(scopeQuery)
	}

	search := bleve.NewSearchRequestOptions(boolQuery, topK, 0, false)
	search.Fields = []string{"content"}
	result, err := s.bleveIndex.Search(search)
	if err != nil {
		return nil, fmt.Errorf("bleve query: %w", err)
	}

	hits := make([]rankedHit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		hits = append(hits, rankedHit{
			docID: hit.ID,
			score: hit.Score,
		})
	}
	return hits, nil
}

func buildScopeQuery(sessionID string, libraryIDs []uint) query.Query {
	var parts []query.Query
	if sessionID != "" {
		sessionQuery := bleve.NewTermQuery(sessionID)
		sessionQuery.SetField("sessionId")
		parts = append(parts, sessionQuery)
	}
	for _, libraryID := range libraryIDs {
		value := float64(libraryID)
		minInclusive := true
		maxInclusive := true
		libQuery := bleve.NewNumericRangeInclusiveQuery(&value, &value, &minInclusive, &maxInclusive)
		libQuery.SetField("memoryLibraryId")
		parts = append(parts, libQuery)
	}
	if len(parts) == 0 {
		return nil
	}
	return bleve.NewDisjunctionQuery(parts...)
}

func chromemMetadata(doc indexedDoc) map[string]string {
	meta := map[string]string{
		"sourceKind": doc.SourceKind,
	}
	if doc.SessionID != "" {
		meta["sessionId"] = doc.SessionID
	}
	if doc.MemoryEntryID > 0 {
		meta["memoryEntryId"] = strconv.FormatUint(uint64(doc.MemoryEntryID), 10)
	}
	if doc.MemoryLibraryID > 0 {
		meta["memoryLibraryId"] = strconv.FormatUint(uint64(doc.MemoryLibraryID), 10)
	}
	return meta
}

func bleveDoc(doc indexedDoc) map[string]interface{} {
	out := map[string]interface{}{
		"content":    doc.Content,
		"sourceKind": doc.SourceKind,
	}
	if doc.SessionID != "" {
		out["sessionId"] = doc.SessionID
	}
	if doc.MemoryEntryID > 0 {
		out["memoryEntryId"] = float64(doc.MemoryEntryID)
	}
	if doc.MemoryLibraryID > 0 {
		out["memoryLibraryId"] = float64(doc.MemoryLibraryID)
	}
	return out
}

func parseUintMeta(value string) uint {
	n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return uint(n)
}
