package handlers

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	database "pkgs/db"
	"runtime"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type EditorInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Command     string `json:"command"`
	IsAvailable bool   `json:"isAvailable"`
}

type EditorHandler struct {
	db *gorm.DB
}

func NewEditorHandler(db *gorm.DB) *EditorHandler {
	return &EditorHandler{db: db}
}

var supportedEditors = []EditorInfo{
	{ID: "vscode", Name: "VS Code", Command: "code"},
	{ID: "cursor", Name: "Cursor", Command: "cursor"},
	{ID: "sublime", Name: "Sublime Text", Command: "subl"},
	{ID: "zed", Name: "Zed", Command: "zed"},
	{ID: "intellij", Name: "IntelliJ IDEA", Command: "idea"},
	{ID: "pycharm", Name: "PyCharm", Command: "pycharm"},
	{ID: "webstorm", Name: "WebStorm", Command: "webstorm"},
	{ID: "custom", Name: "自定义...", Command: ""},
}

// GetEditors 获取支持的编辑器列表及其可用性
func (h *EditorHandler) GetEditors(c *gin.Context) {
	editors := make([]EditorInfo, len(supportedEditors))
	copy(editors, supportedEditors)

	for i := range editors {
		_, err := exec.LookPath(editors[i].Command)
		editors[i].IsAvailable = (err == nil)
	}

	c.JSON(http.StatusOK, editors)
}

// OpenProject 在编辑器中打开项目
func (h *EditorHandler) OpenProject(c *gin.Context) {
	var req struct {
		EditorID string `json:"editorId"`
		Path     string `json:"path"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	// 如果未指定路径，使用当前工作目录
	if req.Path == "" {
		var err error
		req.Path, err = os.Getwd()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取当前工作目录"})
			return
		}
	}

	// 如果未指定编辑器，从配置中获取默认编辑器
	if req.EditorID == "" {
		if config, err := database.GetGlobalConfigByKey(h.db, "default_editor"); err == nil {
			req.EditorID = config.Value
		}
	}

	// 查找编辑器命令
	var command string
	if req.EditorID == "custom" {
		if config, err := database.GetGlobalConfigByKey(h.db, "custom_editor_command"); err == nil {
			command = config.Value
		}
	} else {
		for _, e := range supportedEditors {
			if e.ID == req.EditorID {
				command = e.Command
				break
			}
		}
	}

	if command == "" && req.EditorID != "custom" {
		// 默认使用 code
		command = "code"
	}

	// 处理 macOS 应用包路径（如果需要直接打开 .app）
	if runtime.GOOS == "darwin" && !filepath.IsAbs(command) {
		// 某些编辑器在 macOS 上可能需要特殊处理，但 LookPath 通常能解决
	}

	path := strconv.Quote(req.Path)

	cmd := exec.Command(command, path)
	if err := cmd.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "启动编辑器失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已尝试打开编辑器"})
}

// OpenFolderInFileManager 使用系统文件管理器打开目录
func (h *EditorHandler) OpenFolderInFileManager(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	if req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少路径"})
		return
	}

	targetPath := req.Path
	info, err := os.Stat(targetPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径不存在: " + err.Error()})
		return
	}
	if !info.IsDir() {
		targetPath = filepath.Dir(targetPath)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", targetPath)
	case "windows":
		cmd = exec.Command("explorer", targetPath)
	default:
		cmd = exec.Command("xdg-open", targetPath)
	}

	if err := cmd.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "启动文件管理器失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已尝试在文件管理器中打开"})
}
