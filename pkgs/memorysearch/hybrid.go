package memorysearch

import (
	"sort"
	"strconv"
)

const rrfK = 60

type rankedHit struct {
	docID string
	score float64
}

func reciprocalRankFusionLists(lists [][]rankedHit, topK int) []rankedHit {
	if topK <= 0 {
		topK = 8
	}
	scores := make(map[string]float64)
	for _, list := range lists {
		for rank, hit := range list {
			scores[hit.docID] += 1.0 / float64(rrfK+rank+1)
		}
	}
	if len(scores) == 0 {
		return nil
	}
	out := make([]rankedHit, 0, len(scores))
	for docID, score := range scores {
		out = append(out, rankedHit{docID: docID, score: score})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].score > out[j].score
	})
	if len(out) > topK {
		out = out[:topK]
	}
	return out
}

func reciprocalRankFusion(semantic, keyword []rankedHit, topK int) []rankedHit {
	return reciprocalRankFusionLists([][]rankedHit{semantic, keyword}, topK)
}

func matchesScope(sessionID string, libraryIDs []uint, meta map[string]string) bool {
	if meta == nil {
		return false
	}
	if sessionID != "" && meta["sessionId"] == sessionID {
		return true
	}
	if len(libraryIDs) == 0 {
		return false
	}
	libID, err := strconv.ParseUint(meta["memoryLibraryId"], 10, 64)
	if err != nil || libID == 0 {
		return false
	}
	for _, allowed := range libraryIDs {
		if uint(libID) == allowed {
			return true
		}
	}
	return false
}
