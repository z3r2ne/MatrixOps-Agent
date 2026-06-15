package sessionmemory

import "matrixops-agent/types"

func CloneMemoryEntries(entries []*types.MemoryEntry) []*types.MemoryEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]*types.MemoryEntry, 0, len(entries))
	for _, e := range entries {
		if e != nil {
			c := *e
			out = append(out, &c)
		}
	}
	return out
}

func CloneMemoryEntry(entry *types.MemoryEntry) *types.MemoryEntry {
	if entry == nil {
		return nil
	}
	c := *entry
	return &c
}
