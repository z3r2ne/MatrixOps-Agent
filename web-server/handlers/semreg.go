package handlers

import (
	"net/http"
	"strings"
	"sync"

	taskrunner "web-server/services/task_runner"
	semregrunner "web-server/services/semreg_runner"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SemregHandler struct {
	db      *gorm.DB
	wsHub   taskrunner.WSHub
	runner  *semregrunner.Runner
	manager *semregrunner.Manager
	initErr error
	initMu  sync.Once
}

func NewSemregHandler(db *gorm.DB, wsHub taskrunner.WSHub) *SemregHandler {
	return &SemregHandler{db: db, wsHub: wsHub}
}

func (h *SemregHandler) ensureInit() {
	h.initMu.Do(func() {
		h.runner, h.initErr = semregrunner.NewRunner(h.wsHub)
		if h.initErr == nil {
			h.manager = semregrunner.NewManager(h.runner)
		}
	})
}

func (h *SemregHandler) GetScenarios(c *gin.Context) {
	h.ensureInit()
	if h.initErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.initErr.Error()})
		return
	}
	scenarios, err := h.runner.ListScenarios()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"scenarios": scenarios})
}

func (h *SemregHandler) GetStatus(c *gin.Context) {
	h.ensureInit()
	if h.initErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.initErr.Error()})
		return
	}
	cfg := semregrunner.RunConfig{
		WorkDir:     strings.TrimSpace(c.Query("workDir")),
		ProjectID:   strings.TrimSpace(c.Query("projectId")),
		WorkspaceID: parseUintQuery(c.Query("workspaceId")),
	}
	c.JSON(http.StatusOK, h.runner.EnvironmentStatus(cfg))
}

type startSemregRunRequest struct {
	Tiers       []string `json:"tiers"`
	ScenarioIDs []string `json:"scenarioIds"`
	WorkDir     string   `json:"workDir"`
	WorkspaceID uint     `json:"workspaceId"`
	ProjectID   string   `json:"projectId"`
}

func (h *SemregHandler) StartRun(c *gin.Context) {
	h.ensureInit()
	if h.initErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.initErr.Error()})
		return
	}
	var req startSemregRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	cfg := semregrunner.RunConfig{
		Tiers:       req.Tiers,
		ScenarioIDs: req.ScenarioIDs,
		WorkDir:     strings.TrimSpace(req.WorkDir),
		WorkspaceID: req.WorkspaceID,
		ProjectID:   strings.TrimSpace(req.ProjectID),
	}
	if len(cfg.Tiers) == 0 {
		cfg.Tiers = []string{"L0", "L1", "L2"}
	}
	report, err := h.manager.StartRun(h.db, cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, report)
}

func (h *SemregHandler) GetRun(c *gin.Context) {
	h.ensureInit()
	if h.initErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.initErr.Error()})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	report, ok := h.manager.GetRun(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *SemregHandler) CancelRun(c *gin.Context) {
	h.ensureInit()
	if h.initErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.initErr.Error()})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if !h.manager.CancelRun(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found or already completed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type bootstrapSemregRequest struct {
	WorkspaceID uint `json:"workspaceId"`
}

func (h *SemregHandler) Bootstrap(c *gin.Context) {
	h.ensureInit()
	if h.initErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.initErr.Error()})
		return
	}
	var req bootstrapSemregRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	if req.WorkspaceID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspaceId 不能为空"})
		return
	}
	result, err := semregrunner.BootstrapTestWorkspace(h.db, req.WorkspaceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func parseUintQuery(raw string) uint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	var value uint
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0
		}
		value = value*10 + uint(ch-'0')
	}
	return value
}
