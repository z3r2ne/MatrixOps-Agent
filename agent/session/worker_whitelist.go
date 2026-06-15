package session

import (
	"sort"
	"strings"

	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/gorm"
)

func loadCallableWorkerNames(db *gorm.DB) []string {
	if db == nil {
		return nil
	}
	workers, err := database.GetAllWorkers(db)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(workers))
	seen := make(map[string]struct{}, len(workers))
	for _, worker := range workers {
		if worker.Hidden || models.IsCompactionWorker(worker.Name) {
			continue
		}
		name := strings.TrimSpace(worker.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
