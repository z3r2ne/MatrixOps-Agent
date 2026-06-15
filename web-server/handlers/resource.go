package handlers

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	database "pkgs/db"
	"pkgs/skillfs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ResourceHandler 资源处理器
type ResourceHandler struct {
	db *gorm.DB
}

// NewResourceHandler 创建资源处理器
func NewResourceHandler(db *gorm.DB) *ResourceHandler {
	return &ResourceHandler{db: db}
}

// GetResources 获取资源列表（file / branch）
func (h *ResourceHandler) GetResources(c *gin.Context) {
	projectID := c.Query("projectId")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "projectId is required"})
		return
	}

	var pid uint
	if _, err := fmt.Sscanf(projectID, "%d", &pid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid projectId"})
		return
	}

	project, err := database.GetProjectByID(h.db, pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	resources := []map[string][]string{}

	// command resources
	resources = append(resources, map[string][]string{"command": {"review"}})

	// file resources
	fileKeys := []string{}
	if entries, err := os.ReadDir(project.Path); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if name == ".git" {
				continue
			}
			fileKeys = append(fileKeys, name)
		}
	}
	resources = append(resources, map[string][]string{"file": fileKeys})

	// branch resources
	branchKeys := []string{}
	gitDir := filepath.Join(project.Path, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		cmd := exec.Command("git", "branch", "--format=%(refname:short)|%(HEAD)")
		cmd.Dir = project.Path
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				parts := strings.Split(line, "|")
				name := parts[0]
				branchKeys = append(branchKeys, name)
			}
		}
	}
	resources = append(resources, map[string][]string{"branch": branchKeys})

	// worker resources
	workerKeys := []string{}
	if workers, err := database.GetAllWorkers(h.db); err == nil {
		for _, worker := range workers {
			if worker.Name == "" {
				continue
			}
			workerKeys = append(workerKeys, worker.Name)
		}
	}
	resources = append(resources, map[string][]string{"worker": workerKeys})

	// skill resources
	skillKeys := []string{}
	if skills, err := skillfs.ListInstalledSkills(); err == nil {
		for _, skill := range skills {
			if strings.TrimSpace(skill.Name) == "" {
				continue
			}
			skillKeys = append(skillKeys, skill.Name)
		}
	}
	resources = append(resources, map[string][]string{"skill": skillKeys})

	c.JSON(http.StatusOK, resources)
}

// SearchFiles 搜索项目文件（当前工作区）
func (h *ResourceHandler) SearchFiles(c *gin.Context) {
	projectID := c.Query("projectId")
	query := strings.TrimSpace(c.Query("query"))
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "projectId is required"})
		return
	}
	if query == "" {
		c.JSON(http.StatusOK, []string{})
		return
	}

	var pid uint
	if _, err := fmt.Sscanf(projectID, "%d", &pid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid projectId"})
		return
	}

	project, err := database.GetProjectByID(h.db, pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	results := []string{}
	_ = filepath.WalkDir(project.Path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(project.Path, path)
		if relErr != nil {
			return nil
		}
		if strings.Contains(strings.ToLower(rel), strings.ToLower(query)) {
			results = append(results, rel)
			if len(results) >= 200 {
				return filepath.SkipDir
			}
		}
		return nil
	})

	c.JSON(http.StatusOK, results)
}
