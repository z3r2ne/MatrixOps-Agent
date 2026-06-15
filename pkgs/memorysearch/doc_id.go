package memorysearch

import "fmt"

const collectionName = "memory"

func memoryLibraryDocID(libraryID uint) string {
	return fmt.Sprintf("ml:%d", libraryID)
}
