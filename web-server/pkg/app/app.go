package app

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"

	builtinworkers "matrixops.local/core_agent/workersv2/builtin"
	builtinskills "matrixops-agent/skills/builtin"
	"matrixops/pkg/repository"
	"matrixops/pkg/service/git"
	"matrixops/pkg/service/process"
	"matrixops/pkg/service/terminal"
	"matrixops/pkg/service/worker"
	"matrixops/services"
	database "pkgs/db"
	"pkgs/llmheaders"
	"pkgs/memorysearch"
	mcppkg "pkgs/mcp"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type AppOptions struct {
	Quiet     bool
	LogWriter io.Writer
}

// App 应用程序依赖容器
type App struct {
	// 数据库
	DB *gorm.DB

	// Repository 层
	TaskRepo      repository.TaskRepository
	ExecutionRepo repository.ExecutionRepository
	ProjectRepo   repository.ProjectRepository
	WorkspaceRepo repository.WorkspaceRepository
	ConfigRepo    repository.ConfigRepository

	// Service 层
	GitService    *git.Service
	WorkerService *worker.Service
	ProcessMgr    *process.Manager
	TerminalMgr   *terminal.Manager
}

// NewApp 创建应用程序实例
func NewApp() (*App, error) {
	return NewAppWithOptions(AppOptions{})
}

// NewAppWithOptions 创建应用程序实例，可控制内部日志输出行为。
func NewAppWithOptions(opts AppOptions) (*App, error) {
	logWriter := opts.LogWriter
	if logWriter == nil {
		logWriter = os.Stdout
	}
	if opts.Quiet {
		logWriter = io.Discard
	}

	// 初始化数据库
	dbPath, err := database.DBPath()
	if err != nil {
		return nil, err
	}
	log.Printf("数据库路径: %s", dbPath)

	newLogger := logger.New(
		log.New(logWriter, "\r\n", log.LstdFlags),
		logger.Config{
			LogLevel:                  logger.Warn, // 你想要的 warn
			IgnoreRecordNotFoundError: true,        // 关键：不打印 record not found
			Colorful:                  true,
		},
	)
	// 打开数据库连接
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return nil, err
	}
	if err := configureSQLite(db); err != nil {
		return nil, err
	}
	if err := database.InitDB(db, builtinworkers.ReadAll(), builtinskills.ReadAll()); err != nil {
		return nil, err
	}
	llmheaders.InitFromDatabase(db)
	mcpManager := mcppkg.InitManager(db)
	mcpManager.ConnectAll(context.Background())
	if err := memorysearch.InitStore(); err != nil {
		return nil, fmt.Errorf("init memory search store: %w", err)
	}
	services.InitMemoryLibraryIndexWorker(db)
	services.InitReminderWorker(db)

	// 创建 Repository 层
	taskRepo := repository.NewTaskRepository(db)
	executionRepo := repository.NewExecutionRepository(db)
	projectRepo := repository.NewProjectRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	configRepo := repository.NewConfigRepository(db)

	// 创建 Service 层
	gitService := git.NewService(projectRepo, executionRepo, taskRepo)
	workerService := worker.NewService(projectRepo)
	processManager := process.NewManager()
	terminalManager := terminal.NewManager(db)

	log.Println("应用程序依赖初始化完成")

	return &App{
		DB:            db,
		TaskRepo:      taskRepo,
		ExecutionRepo: executionRepo,
		ProjectRepo:   projectRepo,
		WorkspaceRepo: workspaceRepo,
		ConfigRepo:    configRepo,
		GitService:    gitService,
		WorkerService: workerService,
		ProcessMgr:    processManager,
		TerminalMgr:   terminalManager,
	}, nil
}

func configureSQLite(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)
	sqlDB.SetConnMaxIdleTime(0)

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA busy_timeout=10000;",
		"PRAGMA temp_store=MEMORY;",
	}

	for _, stmt := range pragmas {
		if _, execErr := sqlDB.Exec(stmt); execErr != nil {
			return execErr
		}
	}

	return pingSQLite(sqlDB)
}

func pingSQLite(db *sql.DB) error {
	if db == nil {
		return nil
	}
	return db.Ping()
}

// Cleanup 清理资源
func (app *App) Cleanup() {
	log.Println("开始清理应用程序资源...")
	if app.ProcessMgr != nil {
		app.ProcessMgr.CleanupAll()
	}
	if app.TerminalMgr != nil {
		app.TerminalMgr.CleanupAll()
	}
	log.Println("应用程序资源清理完成")
}
