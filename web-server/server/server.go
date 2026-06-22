package server

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"matrixops/handlers"
	"matrixops/pkg/app"
	"matrixops/services"
	"pkgs/db/models"
	"pkgs/shellutil"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// ServerConfig 服务器配置
type ServerConfig struct {
	Host              string
	Port              string
	EmbeddedFiles     *embed.FS // 静态文件（可选）
	EnablePprof       bool
	PprofAddr         string
	EnablePprofDump   bool
	PprofDumpDir      string
	PprofDumpInterval time.Duration
}

// Start 启动 Web 服务器
func Start(config ServerConfig) error {
	shellutil.ApplyLoginShellEnv()

	// 初始化应用程序
	application, err := app.NewApp()
	if err != nil {
		return fmt.Errorf("应用程序初始化失败: %w", err)
	}

	// 清理上次运行时未完成的任务和执行记录
	cleanupStaleTasksAndExecutions(application)

	// 创建路由
	r := newRouter(application, config.EmbeddedFiles)

	// 构建地址
	addr := fmt.Sprintf("%s:%s", config.Host, config.Port)

	// 创建 HTTP 服务器
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	pprofDumpWorker, err := startPprofDumpWorker(config)
	if err != nil {
		return fmt.Errorf("启动 pprof 自动落盘失败: %w", err)
	}

	var pprofSrv *http.Server
	if config.EnablePprof {
		pprofAddr := strings.TrimSpace(config.PprofAddr)
		if pprofAddr == "" {
			pprofAddr = "localhost:6060"
		}

		pprofSrv = &http.Server{
			Addr:    pprofAddr,
			Handler: newPprofMux(),
		}
	}

	// 启动服务器
	go func() {
		log.Printf("🚀 服务器启动在 http://%s:%s", config.Host, config.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()
	if pprofSrv != nil {
		go func() {
			log.Printf("📈 pprof 已启动: http://%s/debug/pprof/", pprofSrv.Addr)
			if err := pprofSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("pprof 服务启动失败: %v", err)
			}
		}()
	}

	// 监听系统信号，优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("收到退出信号，开始优雅关闭...")

	// 清理应用程序资源
	application.Cleanup()

	// 关闭服务器，最多等待 10 秒
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("服务器关闭出错: %v", err)
		return err
	}
	if pprofDumpWorker != nil {
		pprofDumpWorker.Stop()
	}
	if pprofSrv != nil {
		if err := pprofSrv.Shutdown(ctx); err != nil {
			log.Printf("pprof 服务关闭出错: %v", err)
			return err
		}
	}

	log.Println("✅ 服务器已安全关闭")
	return nil
}

func newPprofMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	return mux
}

// cleanupStaleTasksAndExecutions 清理上次运行时未完成的任务和执行记录
func cleanupStaleTasksAndExecutions(app *app.App) {
	now := time.Now()

	// 1. 将所有 "active" 状态的任务改为 "failed"
	taskResult := app.DB.Model(&models.Task{}).
		Where("status = ?", "active").
		Updates(map[string]interface{}{
			"status":     "failed",
			"updated_at": now,
		})
	if taskResult.Error != nil {
		log.Printf("[Cleanup] 更新任务状态失败: %v", taskResult.Error)
	} else if taskResult.RowsAffected > 0 {
		log.Printf("[Cleanup] 已将 %d 个运行中的任务标记为失败（服务器重启）", taskResult.RowsAffected)
	}

	// 2. 将所有 "running" 状态的执行记录改为 "failed"
	execResult := app.DB.Model(&models.TaskExecution{}).
		Where("status = ?", "running").
		Updates(map[string]interface{}{
			"status":      "failed",
			"finished_at": now,
		})
	if execResult.Error != nil {
		log.Printf("[Cleanup] 更新执行记录状态失败: %v", execResult.Error)
	} else if execResult.RowsAffected > 0 {
		log.Printf("[Cleanup] 已将 %d 个运行中的执行记录标记为失败（服务器重启）", execResult.RowsAffected)
	}
}

func newRouter(app *app.App, embeddedFiles *embed.FS) *gin.Engine {
	r := gin.Default()
	configureCORS(r)
	registerHealth(r)
	registerAPI(r, app)
	registerStatic(r, embeddedFiles)
	return r
}

func configureCORS(r *gin.Engine) {
	config := cors.DefaultConfig()
	allowAll := strings.EqualFold(os.Getenv("CORS_ALLOW_ALL"), "true")
	if allowAll {
		config.AllowAllOrigins = true
	} else {
		originsEnv := strings.TrimSpace(os.Getenv("CORS_ALLOW_ORIGINS"))
		if originsEnv != "" {
			parts := strings.Split(originsEnv, ",")
			for i, origin := range parts {
				parts[i] = strings.TrimSpace(origin)
			}
			config.AllowOrigins = parts
		} else {
			config.AllowOrigins = []string{
				"http://localhost:3000",
				"http://127.0.0.1:3000",
				"http://localhost:3001",
				"http://127.0.0.1:3001",
				"http://localhost:3002",
				"http://127.0.0.1:3002",
				"http://localhost:5173",
				"http://127.0.0.1:5173",
			}
		}
	}
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	r.Use(cors.New(config))
}

func registerHealth(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"message": "MatrixOps API",
		})
	})
}

func registerAPI(r *gin.Engine, app *app.App) {
	api := r.Group("/api")
	{
		providerHandler := handlers.NewProviderHandler(app.DB)
		providers := api.Group("/providers")
		{
			providers.GET("", providerHandler.GetProviders)
			providers.PUT("/:name", providerHandler.UpdateProvider)
		}

		workerHandler := handlers.NewWorkerHandler(app.DB)
		workers := api.Group("/workers")
		{
			workers.GET("", workerHandler.GetWorkers)
			workers.POST("", workerHandler.CreateWorker)
			workers.POST("/bulk-apply-config", workerHandler.BulkApplyConfig)
			workers.POST("/export", workerHandler.ExportWorkers)
			workers.POST("/import", workerHandler.ImportWorkers)
			workers.POST("/restore-defaults", workerHandler.RestoreDefaultWorkers)
			workers.PUT("/:id", workerHandler.UpdateWorker)
			workers.DELETE("/:id", workerHandler.DeleteWorker)
		}

		skillHandler := handlers.NewSkillHandler(app.DB)
		skillSources := api.Group("/skill-sources")
		{
			skillSources.GET("", skillHandler.ListSources)
			skillSources.POST("", skillHandler.CreateSource)
			skillSources.PUT("/:id", skillHandler.UpdateSource)
			skillSources.DELETE("/:id", skillHandler.DeleteSource)
			skillSources.POST("/:id/sync", skillHandler.SyncSource)
		}

		openUIApplicationHandler := handlers.NewOpenUIApplicationHandler(app.DB)
		uiOpen := api.Group("/ui/open")
		{
			uiOpen.GET("", openUIApplicationHandler.GetOpen)
			uiOpen.POST("/workspaces/:id", openUIApplicationHandler.PostOpenWorkspace)
			uiOpen.DELETE("/workspaces/:id", openUIApplicationHandler.DeleteOpenWorkspace)
			uiOpen.POST("/last-closed-workspace", openUIApplicationHandler.PostLastClosedWorkspace)
			uiOpen.POST("/projects/:id", openUIApplicationHandler.PostOpenProject)
			uiOpen.DELETE("/projects/:id", openUIApplicationHandler.DeleteOpenProject)
		}

		skills := api.Group("/skills")
		{
			skills.GET("", skillHandler.ListSkills)
			skills.POST("/install", skillHandler.InstallSkill)
			skills.POST("/uninstall", skillHandler.UninstallSkill)
		}

		mcpHandler := handlers.NewMcpHandler(app.DB)
		mcpServers := api.Group("/mcp-servers")
		{
			mcpServers.GET("", mcpHandler.ListServers)
			mcpServers.POST("", mcpHandler.CreateServer)
			mcpServers.PUT("/:id", mcpHandler.UpdateServer)
			mcpServers.DELETE("/:id", mcpHandler.DeleteServer)
			mcpServers.POST("/:id/reconnect", mcpHandler.ReconnectServer)
			mcpServers.GET("/:id/tools", mcpHandler.ListServerTools)
		}

		searchConfigHandler := handlers.NewSearchConfigHandler(app.DB)
		searchConfigs := api.Group("/search-configs")
		{
			searchConfigs.GET("", searchConfigHandler.ListConfigs)
			searchConfigs.POST("", searchConfigHandler.CreateConfig)
			searchConfigs.PUT("/:id", searchConfigHandler.UpdateConfig)
			searchConfigs.DELETE("/:id", searchConfigHandler.DeleteConfig)
			searchConfigs.POST("/:id/test", searchConfigHandler.TestConfig)
		}

		embeddingConfigHandler := handlers.NewEmbeddingConfigHandler(app.DB)
		embeddingConfigs := api.Group("/embedding-configs")
		{
			embeddingConfigs.GET("", embeddingConfigHandler.ListConfigs)
			embeddingConfigs.POST("", embeddingConfigHandler.CreateConfig)
			embeddingConfigs.PUT("/:id", embeddingConfigHandler.UpdateConfig)
			embeddingConfigs.DELETE("/:id", embeddingConfigHandler.DeleteConfig)
			embeddingConfigs.POST("/:id/test", embeddingConfigHandler.TestConfig)
			embeddingConfigs.GET("/:id/health", embeddingConfigHandler.HealthCheck)
		}

		memoryLibrarySearchIndexHandler := handlers.NewMemoryLibrarySearchIndexHandler(app.DB)

		workspaceHandler := handlers.NewWorkspaceHandler(app.DB)
		workspaces := api.Group("/workspaces")
		{
			workspaces.GET("", workspaceHandler.GetWorkspaces)
			workspaces.POST("", workspaceHandler.CreateWorkspace)

			workspace := workspaces.Group("/:id")
			{
				workspace.GET("", workspaceHandler.GetWorkspace)
				workspace.PUT("", workspaceHandler.UpdateWorkspace)
				workspace.DELETE("", workspaceHandler.DeleteWorkspace)
				workspace.POST("/activate", workspaceHandler.SetActiveWorkspace)

				projectHandler := handlers.NewProjectHandler(app.DB)
				workspace.GET("/projects", projectHandler.GetProjects)
				workspace.POST("/projects", projectHandler.CreateProject)
				workspace.POST("/projects/:projectId", projectHandler.AddProjectToWorkspace)        // 添加已存在的项目到工作区
				workspace.DELETE("/projects/:projectId", projectHandler.RemoveProjectFromWorkspace) // 从工作区移除项目
			}
		}

		projectHandler := handlers.NewProjectHandler(app.DB)
		projects := api.Group("/projects")
		{
			projects.GET("", projectHandler.GetAllProjects)           // 获取所有项目
			projects.POST("", projectHandler.CreateStandaloneProject) // 创建独立项目（不关联工作区）
			projects.GET("/:id", projectHandler.GetProject)
			projects.PUT("/:id", projectHandler.UpdateProject)
			projects.DELETE("/:id", projectHandler.DeleteProject)

			// Git 相关 API
			gitHandler := handlers.NewGitHandler(app.DB)
			projects.GET("/:id/git/check", gitHandler.CheckGitRepo)
			projects.POST("/:id/git/init", gitHandler.InitGitRepo)
			projects.GET("/:id/branches", gitHandler.GetBranches)
			projects.GET("/:id/branch/current", gitHandler.GetCurrentBranch)
			projects.GET("/:id/branch/default", gitHandler.GetDefaultBranch)
			projects.GET("/:id/worktrees", gitHandler.GetWorktrees)
			projects.POST("/:id/worktrees", gitHandler.CreateWorktree)
			projects.DELETE("/:id/worktrees", gitHandler.DeleteWorktree)
		}

		memoryLibraryHandler := handlers.NewMemoryLibraryHandler(app.DB)
		memoryLibraries := api.Group("/memory-libraries")
		{
			memoryLibraries.GET("", memoryLibraryHandler.GetMemoryLibraries)
			memoryLibraries.POST("", memoryLibraryHandler.CreateMemoryLibrary)
			memoryLibraries.GET("/:id", memoryLibraryHandler.GetMemoryLibrary)
			memoryLibraries.PUT("/:id", memoryLibraryHandler.UpdateMemoryLibrary)
			memoryLibraries.POST("/:id/promote", memoryLibraryHandler.PromoteMemoryLibrary)
			memoryLibraries.DELETE("/:id", memoryLibraryHandler.DeleteMemoryLibrary)
			memoryLibraries.GET("/:id/search-index/status", memoryLibrarySearchIndexHandler.GetStatus)
			memoryLibraries.POST("/:id/search-index/rebuild", memoryLibrarySearchIndexHandler.Rebuild)
		}

		resourceHandler := handlers.NewResourceHandler(app.DB)
		resources := api.Group("/resources")
		{
			resources.GET("", resourceHandler.GetResources)
			resources.GET("/search", resourceHandler.SearchFiles)
		}

		toolHandler := handlers.NewToolHandler(app.DB)
		api.GET("/tools", toolHandler.GetTools)

		taskHandler := handlers.NewTaskHandler(app.DB)
		sessionHandler := handlers.NewSessionHandler(app.DB)
		gitHandler := handlers.NewGitHandler(app.DB)

		api.POST("/temp-uploads", taskHandler.UploadTempFiles)
		api.GET("/temp-uploads", taskHandler.GetTempFile)

		tasks := api.Group("/tasks")
		{
			tasks.GET("/:id", taskHandler.GetTask)
			tasks.PUT("/:id", taskHandler.UpdateTask)
			tasks.DELETE("/:id", taskHandler.DeleteTask)
			tasks.GET("/:id/executions", taskHandler.GetTaskExecutions)
			tasks.GET("/:id/executions/:execId", taskHandler.GetTaskExecution)
			tasks.GET("/:id/executions/:execId/logs", taskHandler.GetExecutionLogs)
			tasks.GET("/:id/logs", taskHandler.GetTaskLogs)
			tasks.GET("/:id/logsv2", taskHandler.GetTaskLogsV2)
			tasks.POST("/:id/retry-last-user-message", taskHandler.RetryLastUserMessage)
			tasks.GET("/:id/prompt", taskHandler.GetTaskPrompt)
			tasks.GET("/:id/plan", taskHandler.GetTaskPlan)
			tasks.GET("/:id/queue", taskHandler.GetTaskQueue)
			tasks.PUT("/:id/queue", taskHandler.UpdateTaskQueue)
			tasks.POST("/:id/queue/:itemId/send-next", taskHandler.SendNextTaskQueueItem)
			tasks.POST("/:id/user-input-files", taskHandler.UploadTaskUserInputFiles)
			tasks.GET("/:id/user-input-files", taskHandler.GetTaskUserInputFile)
			tasks.GET("/:id/filesystem/roots", taskHandler.GetTaskFilesystemRoots)
			tasks.GET("/:id/filesystem/list", taskHandler.ListTaskFilesystem)
			tasks.GET("/:id/filesystem/read", taskHandler.ReadTaskFilesystem)
			tasks.PUT("/:id/filesystem/write", taskHandler.WriteTaskFilesystem)

			// Git 操作
			tasks.GET("/:id/git/diff", gitHandler.GetTaskDiff)
			tasks.GET("/:id/git/timeline", gitHandler.GetTaskGitTimeline)
			tasks.GET("/:id/git/state", gitHandler.GetCurrentGitState)
			tasks.GET("/:id/executions/:execId/git/state", gitHandler.GetExecutionGitState)
			tasks.POST("/:id/git/commit", gitHandler.GitCommit)
			tasks.POST("/:id/git/merge", gitHandler.GitMerge)
			tasks.POST("/:id/git/restore", gitHandler.RestoreSnapshot)
			tasks.POST("/:id/git/restore-ref", gitHandler.GitRestoreWorktreeRef)
			tasks.POST("/:id/git/snapshot/apply", gitHandler.ApplyTaskSnapshot)
		}

		sessions := api.Group("/sessions")
		{
			sessions.GET("/:id", sessionHandler.GetSession)
			sessions.GET("/:id/logsv2", sessionHandler.GetSessionLogsV2)
			sessions.GET("/:id/export", sessionHandler.ExportSessionTransfer)
			sessions.POST("/:id/import", sessionHandler.ImportSessionTransfer)
			sessions.GET("/:id/prompt", sessionHandler.GetSessionPrompt)
			sessions.GET("/:id/context", sessionHandler.GetSessionContext)
			sessions.GET("/:id/memory", sessionHandler.GetSessionMemory)
			sessions.POST("/:id/skills/remove", sessionHandler.RemoveSessionSkill)
			sessions.POST("/:id/memory/entries", sessionHandler.CreateSessionMemoryEntry)
			sessions.POST("/:id/memory/compact", sessionHandler.CompactSessionMemory)
			sessions.POST("/:id/memory/entries/compress", sessionHandler.CompressSessionMemoryEntries)
			sessions.POST("/:id/memory/organization/preview", sessionHandler.PreviewSessionMemoryCompaction)
			sessions.POST("/:id/memory/organization/preview/stream", sessionHandler.PreviewSessionMemoryCompactionStream)
			sessions.POST("/:id/memory/organization/apply", sessionHandler.ApplySessionMemoryCompaction)
			sessions.POST("/:id/memory/analysis", sessionHandler.AnalyzeSessionMemory)
			sessions.PATCH("/:id/memory/entries/:entryId", sessionHandler.UpdateSessionMemoryEntry)
			sessions.DELETE("/:id/memory/entries/:entryId", sessionHandler.DeleteSessionMemoryEntry)
			sessions.GET("/:id/logs", taskHandler.GetSessionLogs)
		}

		workspaces.GET("/:id/tasks", taskHandler.GetTasksByWorkspace)
		workspaces.POST("/:id/tasks", taskHandler.CreateWorkspaceTask)
		workspaces.PUT("/:id/tasks/reorder", taskHandler.ReorderWorkspaceTasks)
		workspaces.POST("/:id/tasks/run", taskHandler.RunWorkspaceTask)

		// 命令日志 API
		commandLogHandler := handlers.NewCommandLogHandler(app.DB)
		logs := api.Group("/command-logs")
		{
			logs.GET("", commandLogHandler.GetLogs)
			logs.GET("/stats", commandLogHandler.GetStats)
			logs.GET("/:id", commandLogHandler.GetLog)
			logs.DELETE("/clear", commandLogHandler.ClearOldLogs)
		}

		usageAnalyticsHandler := handlers.NewUsageAnalyticsHandler(app.DB)
		api.GET("/usage/analytics", usageAnalyticsHandler.GetUsageAnalytics)

		// 全局 WebSocket 端点
		globalWSHandler := handlers.NewGlobalWSHandler(app.DB)

		api.GET("/ws", globalWSHandler.Handle)

		terminalHandler := handlers.NewTerminalHandler(app.TerminalMgr)
		terminals := api.Group("/terminals")
		{
			terminals.POST("/sessions", terminalHandler.CreateSession)
			terminals.GET("/sessions/:id", terminalHandler.PollSession)
			terminals.POST("/sessions/:id/input", terminalHandler.WriteSession)
			terminals.POST("/sessions/:id/resize", terminalHandler.ResizeSession)
			terminals.DELETE("/sessions/:id", terminalHandler.CloseSession)
		}

		// 大模型配置 API
		llmHandler := handlers.NewLLMHandler(app.DB)
		llmGroup := api.Group("/llm")
		{
			llmGroup.GET("/configs", llmHandler.GetLLMConfigs)
			llmGroup.GET("/configs/default", llmHandler.GetDefaultLLMConfig)
			llmGroup.GET("/configs/:id", llmHandler.GetLLMConfig)
			llmGroup.POST("/configs", llmHandler.CreateLLMConfig)
			llmGroup.PUT("/configs/:id", llmHandler.UpdateLLMConfig)
			llmGroup.PUT("/configs/:id/set-default", llmHandler.SetDefaultLLMConfig)
			llmGroup.DELETE("/configs/:id", llmHandler.DeleteLLMConfig)
			llmGroup.GET("/models", llmHandler.GetLLMModels)
			llmGroup.POST("/models/preview", llmHandler.PreviewLLMModels)
			llmGroup.POST("/debug", llmHandler.DebugLLM)
			llmGroup.POST("/generate-commit-message", llmHandler.GenerateCommitMessage)
		}

	semregHandler := handlers.NewSemregHandler(app.DB, services.GetGlobalWSHub(app.DB))
		semregGroup := api.Group("/semreg")
		{
			semregGroup.GET("/scenarios", semregHandler.GetScenarios)
			semregGroup.GET("/status", semregHandler.GetStatus)
			semregGroup.POST("/runs", semregHandler.StartRun)
			semregGroup.GET("/runs/:id", semregHandler.GetRun)
			semregGroup.POST("/runs/:id/cancel", semregHandler.CancelRun)
			semregGroup.POST("/bootstrap", semregHandler.Bootstrap)
		}

		// 全局配置 API
		configHandler := handlers.NewConfigHandler(app.DB)
		configGroup := api.Group("/config")
		{
			configGroup.GET("/:key", configHandler.GetConfig)
			configGroup.PUT("/:key", configHandler.UpdateConfig)
			configGroup.GET("/shell/current", configHandler.GetCurrentShell)
			configGroup.GET("/keep-process-alive/status", configHandler.GetKeepProcessAlive)
			configGroup.PUT("/keep-process-alive/status", configHandler.UpdateKeepProcessAlive)
			configGroup.GET("/active-processes/list", configHandler.GetActiveProcesses)
			configGroup.POST("/active-processes/kill", configHandler.KillProcess)
		}

		// 编辑器相关 API
		editorHandler := handlers.NewEditorHandler(app.DB)
		editorGroup := api.Group("/editors")
		{
			editorGroup.GET("", editorHandler.GetEditors)
			editorGroup.POST("/open", editorHandler.OpenProject)
			editorGroup.POST("/open-folder", editorHandler.OpenFolderInFileManager)
		}

		// 模型设置 API
		handlers.SetupModelSettingsRoutes(api, app.DB)

		// 提示词管理 API
		promptHandler := handlers.NewPromptHandler(app.DB)
		promptGroup := api.Group("/prompts")
		{
			// 全局提示词
			promptGroup.GET("/global", promptHandler.GetGlobalPrompt)
			promptGroup.PUT("/global", promptHandler.UpdateGlobalPrompt)

			// 职业管理
			promptGroup.GET("/occupations", promptHandler.GetAllOccupations)
			promptGroup.GET("/occupations/:id", promptHandler.GetOccupation)
			promptGroup.GET("/occupations/code/:code", promptHandler.GetOccupationByCode)
			promptGroup.PUT("/occupations/:id", promptHandler.UpdateOccupation)

			// 项目提示词
			promptGroup.GET("/projects/:id", promptHandler.GetProjectPrompt)
			promptGroup.PUT("/projects/:id", promptHandler.UpdateProjectPrompt)
		}

		// iLink 微信账号 API
		hub := services.GetGlobalWSHub(app.DB)
		ilinkService := services.GetILinkService(app.DB, hub)
		ilinkService.LoadAndStartEnabledAccounts()
		ilinkHandler := handlers.NewILinkHandler(app.DB, ilinkService)
		ilinkGroup := api.Group("/ilink")
		{
			ilinkGroup.GET("/accounts", ilinkHandler.GetAccounts)
			ilinkGroup.GET("/accounts/:id", ilinkHandler.GetAccount)
			ilinkGroup.PUT("/accounts/:id", ilinkHandler.UpdateAccount)
			ilinkGroup.DELETE("/accounts/:id", ilinkHandler.DeleteAccount)
			ilinkGroup.POST("/accounts/:id/start", ilinkHandler.StartAccount)
			ilinkGroup.POST("/accounts/:id/stop", ilinkHandler.StopAccount)
			ilinkGroup.GET("/qrcode", ilinkHandler.FetchQRCode)
			ilinkGroup.GET("/qrcode/status", ilinkHandler.PollQRStatus)
			ilinkGroup.GET("/tasks-for-binding", ilinkHandler.GetTasksForBinding)
		}

		// 测试场景 API
		testHandler := handlers.NewTestHandler(app.DB, hub)
		testGroup := api.Group("/test")
		{
			testGroup.GET("/scenarios", testHandler.ListScenarios)
			testGroup.POST("/workspaces/:id/run", testHandler.RunScenario)
		}
	}
}

func registerStatic(r *gin.Engine, embeddedFiles *embed.FS) {
	var fileSystem http.FileSystem

	if embeddedFiles != nil {
		// 使用嵌入的文件
		staticFS, err := fs.Sub(*embeddedFiles, "web/dist")
		if err != nil {
			log.Printf("静态资源未准备好: %v", err)
			return
		}
		fileSystem = http.FS(staticFS)
		log.Println("📦 使用嵌入的静态文件")
	} else {
		// 从文件系统加载
		// 尝试多个可能的路径
		possiblePaths := []string{
			"web/dist",               // 从 web-server/ 目录启动
			"web-server/web/dist",    // 从项目根目录启动
			"../web-server/web/dist", // 从其他子目录启动
		}

		var distPath string
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				distPath = path
				break
			}
		}

		if distPath == "" {
			log.Printf("⚠️  静态文件目录不存在")
			log.Printf("尝试过的路径:")
			for _, path := range possiblePaths {
				log.Printf("  - %s", path)
			}
			log.Printf("提示: 请先运行 'cd frontend && npm run build'")
			return
		}

		fileSystem = http.Dir(distPath)
		absPath, _ := os.Getwd()
		log.Printf("📁 从文件系统加载静态文件")
		log.Printf("   工作目录: %s", absPath)
		log.Printf("   静态目录: %s", distPath)
	}

	r.NoRoute(func(c *gin.Context) {
		reqPath := c.Request.URL.Path

		// API 路由返回 404
		if strings.HasPrefix(reqPath, "/api") {
			c.Status(http.StatusNotFound)
			return
		}

		// 根路径或其他路径，尝试提供文件
		filePath := strings.TrimPrefix(reqPath, "/")
		if filePath == "" {
			filePath = "index.html"
		}

		// 打开文件
		file, err := fileSystem.Open(filePath)
		if err != nil {
			// 文件不存在，返回 index.html（用于 SPA 路由）
			file, err = fileSystem.Open("index.html")
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			defer file.Close()

			stat, _ := file.Stat()
			http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), file.(io.ReadSeeker))
			return
		}
		defer file.Close()

		// 检查是否是目录
		stat, err := file.Stat()
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		if stat.IsDir() {
			// 如果是目录，返回 index.html
			file.Close()
			file, err = fileSystem.Open("index.html")
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			defer file.Close()
			stat, _ = file.Stat()
			http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), file.(io.ReadSeeker))
			return
		}

		// 提供文件
		http.ServeContent(c.Writer, c.Request, filePath, stat.ModTime(), file.(io.ReadSeeker))
	})
}
