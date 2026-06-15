package llmheaders

import (
	"log"

	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/gorm"
)

// InitFromDatabase loads llm_custom_headers from global config into memory (no-op if missing or invalid).
func InitFromDatabase(db *gorm.DB) {
	if db == nil {
		return
	}
	cfg, err := database.GetGlobalConfigByKey(db, models.ConfigKeyLLMCustomHeaders)
	if err != nil {
		if err := SetFromJSON(""); err != nil {
			log.Printf("llmheaders: clear failed: %v", err)
		}
		return
	}
	if err := SetFromJSON(cfg.Value); err != nil {
		log.Printf("llmheaders: stored %s invalid (%v), ignoring", models.ConfigKeyLLMCustomHeaders, err)
		_ = SetFromJSON("")
	}
}
