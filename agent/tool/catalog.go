package tool

import (
	"sort"

	mcppkg "pkgs/mcp"
	"pkgs/db/models"

	"gorm.io/gorm"
)

type Info struct {
	Name        string `json:"name"`
	VerbosName  string `json:"verbosName"`
	Description string `json:"description"`
	IsMcp       bool   `json:"isMcp,omitempty"`
	McpServer   string `json:"mcpServer,omitempty"`
}

type VisibleCatalogOptions struct {
	WorkerEnabledTools     map[string]struct{}
	HasWorkerEnabledTools  bool
	ProjectToolPermissions map[string]string
	ProjectYoloMode        bool
}

func Catalog() []Info {
	return catalogFromRegistry(NewDefaultRegistryWithQuestion(&DefaultRegistryOptions{}))
}

func CatalogMerged() []Info {
	registry := NewDefaultRegistryWithQuestion(&DefaultRegistryOptions{})
	if manager := mcppkg.GetManager(); manager != nil {
		RegisterMcpTools(registry, manager)
	}
	return catalogFromRegistry(registry)
}

// CatalogForWorkerUI returns built-in tools plus conditional search tools.
func CatalogForWorkerUI(db *gorm.DB) []Info {
	registry := NewDefaultRegistryWithQuestion(&DefaultRegistryOptions{})
	registerCatalogRunWorkerTask(registry)
	RegisterSearchTools(registry, db)
	RegisterMemorySearchTools(registry, db, "")
	return catalogFromRegistry(registry)
}

// CatalogBuiltin returns built-in tools only (excludes MCP tools).
func CatalogBuiltin() []Info {
	infos := Catalog()
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	return infos
}

// CatalogVisible returns tools visible to the AI for the given worker/project constraints.
func CatalogVisible(opts VisibleCatalogOptions) []Info {
	registry := NewDefaultRegistryWithQuestion(&DefaultRegistryOptions{})
	if manager := mcppkg.GetManager(); manager != nil {
		RegisterMcpTools(registry, manager)
	}
	infos := make([]Info, 0)
	for _, name := range registry.Names() {
		if !isToolVisible(name, opts) {
			continue
		}
		instance, err := registry.Get(name)
		if err != nil {
			continue
		}
		info := Info{
			Name:        instance.Name(),
			VerbosName:  instance.VerbosName(),
			Description: instance.Description(),
		}
		if serverName, _, ok := mcppkg.ParseToolFullName(info.Name); ok {
			info.IsMcp = true
			info.McpServer = serverName
		}
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	return infos
}

func isToolVisible(name string, opts VisibleCatalogOptions) bool {
	if opts.HasWorkerEnabledTools && !mcppkg.IsMcpToolFullName(name) && !IsWebSearchTool(name) && !IsMemorySearchTool(name) {
		if _, ok := opts.WorkerEnabledTools[name]; !ok {
			return false
		}
	}
	if !opts.ProjectYoloMode {
		if opts.ProjectToolPermissions != nil &&
			opts.ProjectToolPermissions[name] == models.ProjectToolPermissionDeny {
			return false
		}
	}
	return true
}

func catalogFromRegistry(registry *Registry) []Info {
	infos := make([]Info, 0, len(registry.Names()))
	for _, name := range registry.Names() {
		instance, err := registry.Get(name)
		if err != nil {
			continue
		}
		info := Info{
			Name:        instance.Name(),
			VerbosName:  instance.VerbosName(),
			Description: instance.Description(),
		}
		if serverName, _, ok := mcppkg.ParseToolFullName(info.Name); ok {
			info.IsMcp = true
			info.McpServer = serverName
		}
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	return infos
}

func CatalogNames() []string {
	infos := Catalog()
	names := make([]string, 0, len(infos))
	for _, info := range infos {
		names = append(names, info.Name)
	}
	return names
}

func CatalogNameSet() map[string]struct{} {
	names := CatalogNames()
	result := make(map[string]struct{}, len(names))
	for _, name := range names {
		result[name] = struct{}{}
	}
	return result
}
