package memorysearch

import "matrixops-agent/types"

func filterEntriesPendingSearchArchive(entries []*types.MemoryEntry) []*types.MemoryEntry {
	if len(entries) == 0 {
		return nil
	}
	pending := make([]*types.MemoryEntry, 0, len(entries))
	for _, entry := range entries {
		if entry == nil || entry.SearchArchived {
			continue
		}
		pending = append(pending, entry)
	}
	return pending
}
