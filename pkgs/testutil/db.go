package testutil

import (
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenMemoryDB 打开独立命名的 SQLite 内存库，避免并行测试争用。
func OpenMemoryDB(t *testing.T) *gorm.DB {
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
	return db
}

// MigrateTaskTables 迁移任务与队列相关表。
func MigrateTaskTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&models.Task{}, &models.Part{}, &models.TaskExecution{}); err != nil {
		t.Fatalf("migrate task tables: %v", err)
	}
}

// OpenTaskTestDB 返回已迁移任务表结构的内存库。
func OpenTaskTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenMemoryDB(t)
	MigrateTaskTables(t, db)
	return db
}
