package testutil

import (
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	builtinworkers "matrixops.local/core_agent/workersv2/builtin"
	builtinskills "matrixops-agent/skills/builtin"
	database "pkgs/db"
	"pkgs/db/storage"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SetupFullTestDB 初始化完整 schema（集成测试用）。
func SetupFullTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				LogLevel:                  logger.Warn,
				IgnoreRecordNotFoundError: true,
				Colorful:                  true,
			},
		),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.InitDB(db, builtinworkers.ReadAll(), builtinskills.ReadAll()); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	storage.InitStorage(db)
	database.SetDB(db)
	return db
}
