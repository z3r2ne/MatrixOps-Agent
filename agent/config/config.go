package config

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"matrixops-agent/auth"
	"matrixops-agent/global"
	"matrixops-agent/taskctx"
	"matrixops-agent/util"
	"pkgs/db/models"
)

type Config struct {
	Agent        map[string]AgentConfig
	Mode         map[string]AgentConfig
	Command      map[string]CommandConfig
	Permission   map[string]interface{}
	DefaultAgent string
	Username     string
	Experimental *Experimental
	Compaction   *Compaction
	Model        string
	SmallModel   string
	Provider     map[string]ProviderConfig
	Instructions []string
	Proxy        string
	Snapshot     *bool
	Share        string
}

type Experimental struct {
	OpenTelemetry bool
}

type Compaction struct {
	Auto  *bool
	Prune *bool
}

type AgentConfig struct {
	Name        string
	Model       string
	Temperature float64
	TopP        float64
	Prompt      string
	Tools       map[string]bool
	Disable     bool
	Description string
	Mode        string
	Hidden      bool
	Options     map[string]interface{}
	Color       string
	Steps       int
	MaxSteps    int
	Permission  map[string]interface{}
}

type ProviderConfig struct {
	Options map[string]interface{}
}

type CommandConfig struct {
	Name        string
	Template    string
	Description string
	Agent       string
	Model       string
	Subtask     bool
	MCP         bool
}

func loadConfig(task *models.Task) (*Config, error) {
	if err := global.Init(); err != nil {
		return nil, err
	}
	ctx, err := taskctx.Resolve(task)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{}

	if err := loadWellKnownConfig(result); err != nil {
		return nil, err
	}

	globalConfig, _ := loadConfigFile(globalConfigFile())
	result = mergeConfig(result, globalConfig)

	if path := os.Getenv(global.EnvConfig); path != "" {
		custom, _ := loadConfigFile(path)
		result = mergeConfig(result, custom)
	}

	if !envBool(global.EnvDisableProjectConfig) {
		for _, file := range global.ConfigFileNames {
			found := findUp(file, ctx.WorkDir, ctx.Worktree)
			for i := len(found) - 1; i >= 0; i-- {
				loaded, _ := loadConfigFile(found[i])
				result = mergeConfig(result, loaded)
			}
		}
	}

	if content := os.Getenv(global.EnvConfigContent); content != "" {
		inline := map[string]interface{}{}
		if err := parseJSONC([]byte(content), &inline); err == nil {
			result = mergeConfig(result, inline)
		}
	}

	if result["agent"] == nil {
		result["agent"] = map[string]interface{}{}
	}
	if result["mode"] == nil {
		result["mode"] = map[string]interface{}{}
	}
	if result["command"] == nil {
		result["command"] = map[string]interface{}{}
	}

	dirs := []string{global.Path.Config}
	if !envBool(global.EnvDisableProjectConfig) {
		dirs = append(dirs, findUp(global.ConfigDirName, ctx.WorkDir, ctx.Worktree)...)
	}
	dirs = append(dirs, findUp(global.ConfigDirName, global.Path.Home, global.Path.Home)...)
	if extra := os.Getenv(global.EnvConfigDir); extra != "" {
		dirs = append(dirs, extra)
	}

	for _, dir := range uniqueStrings(dirs) {
		if strings.HasSuffix(dir, global.ConfigDirName) || dir == os.Getenv(global.EnvConfigDir) {
			for _, file := range global.ConfigFileNames {
				loaded, _ := loadConfigFile(filepath.Join(dir, file))
				result = mergeConfig(result, loaded)
			}
		}
		agents := loadAgent(dir)
		if len(agents) > 0 {
			result["agent"] = util.MergeMaps(asMap(result["agent"]), agents)
		}
		modes := loadMode(dir)
		if len(modes) > 0 {
			result["agent"] = util.MergeMaps(asMap(result["agent"]), modes)
		}
		commands := loadCommand(dir)
		if len(commands) > 0 {
			result["command"] = util.MergeMaps(asMap(result["command"]), commands)
		}
	}

	if envPerm := os.Getenv(global.EnvPermission); envPerm != "" {
		override := map[string]interface{}{}
		if err := parseJSONC([]byte(envPerm), &override); err == nil {
			permission := asMap(result["permission"])
			permission = util.MergeMaps(permission, override)
			result["permission"] = permission
		}
	}

	cfg := decodeConfig(result)
	if cfg.Username == "" {
		if currentUser, err := user.Current(); err == nil {
			cfg.Username = currentUser.Username
		}
	}
	return cfg, nil
}

func Get(task *models.Task) (*Config, error) {
	return loadConfig(task)
}

func globalConfigFile() string {
	candidates := make([]string, 0, len(global.ConfigFileNames)+1)
	for _, file := range global.ConfigFileNames {
		candidates = append(candidates, filepath.Join(global.Path.Config, file))
	}
	candidates = append(candidates, filepath.Join(global.Path.Config, "config.json"))
	for _, file := range candidates {
		if _, err := os.Stat(file); err == nil {
			return file
		}
	}
	return candidates[0]
}

func loadConfigFile(path string) (map[string]interface{}, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		return loadYAMLFile(path)
	}
	return loadJSONCFile(path)
}

func loadJSONCFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]interface{}{}, err
	}
	decoded := map[string]interface{}{}
	if err := parseJSONC(data, &decoded); err != nil {
		return map[string]interface{}{}, err
	}
	return decoded, nil
}

func loadYAMLFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]interface{}{}, err
	}
	decoded, err := parseYAML(string(data))
	if err != nil {
		return map[string]interface{}{}, err
	}
	return decoded, nil
}

func mergeConfig(base map[string]interface{}, overlay map[string]interface{}) map[string]interface{} {
	if base == nil {
		base = map[string]interface{}{}
	}
	base = util.MergeMaps(base, overlay)
	return base
}

func findUp(target string, start string, stop string) []string {
	var results []string
	current := start
	for {
		candidate := filepath.Join(current, target)
		if _, err := os.Stat(candidate); err == nil {
			results = append(results, candidate)
		}
		if current == stop {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return results
}

func loadAgent(dir string) map[string]interface{} {
	result := map[string]interface{}{}
	walkDir(dir, func(path string) {
		if !strings.HasSuffix(path, ".md") {
			return
		}
		slashPath := filepath.ToSlash(path)
		idx := strings.Index(slashPath, "/agent/")
		if idx == -1 {
			idx = strings.Index(slashPath, "/agents/")
		}
		if idx == -1 {
			return
		}
		md, err := ParseMarkdown(path)
		if err != nil {
			return
		}
		rel := slashPath[idx+len("/agent/"):]
		if strings.Contains(slashPath[idx:], "/agents/") {
			rel = slashPath[idx+len("/agents/"):]
		}
		name := strings.TrimSuffix(rel, filepath.Ext(rel))
		cfg := map[string]interface{}{"name": name}
		for k, v := range md.Frontmatter {
			cfg[k] = v
		}
		cfg["prompt"] = strings.TrimSpace(md.Content)
		result[name] = cfg
	})
	return result
}

func loadMode(dir string) map[string]interface{} {
	result := map[string]interface{}{}
	for _, folder := range []string{"mode", "modes"} {
		modeDir := filepath.Join(dir, folder)
		entries, err := os.ReadDir(modeDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			path := filepath.Join(modeDir, entry.Name())
			md, err := ParseMarkdown(path)
			if err != nil {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			cfg := map[string]interface{}{"name": name, "mode": "primary"}
			for k, v := range md.Frontmatter {
				cfg[k] = v
			}
			cfg["prompt"] = strings.TrimSpace(md.Content)
			result[name] = cfg
		}
	}
	return result
}

func loadCommand(dir string) map[string]interface{} {
	result := map[string]interface{}{}
	walkDir(dir, func(path string) {
		if !strings.HasSuffix(path, ".md") {
			return
		}
		slashPath := filepath.ToSlash(path)
		idx := strings.Index(slashPath, "/command/")
		if idx == -1 {
			idx = strings.Index(slashPath, "/commands/")
		}
		if idx == -1 {
			return
		}
		md, err := ParseMarkdown(path)
		if err != nil {
			return
		}
		rel := slashPath[idx+len("/command/"):]
		if strings.Contains(slashPath[idx:], "/commands/") {
			rel = slashPath[idx+len("/commands/"):]
		}
		name := strings.TrimSuffix(rel, filepath.Ext(rel))
		cfg := map[string]interface{}{"name": name}
		for k, v := range md.Frontmatter {
			cfg[k] = v
		}
		cfg["template"] = strings.TrimSpace(md.Content)
		result[name] = cfg
	})
	return result
}

func walkDir(root string, fn func(path string)) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if entry.IsDir() {
			walkDir(path, fn)
			continue
		}
		fn(path)
	}
}

func decodeConfig(raw map[string]interface{}) *Config {
	cfg := &Config{
		Agent:      map[string]AgentConfig{},
		Mode:       map[string]AgentConfig{},
		Command:    map[string]CommandConfig{},
		Permission: map[string]interface{}{},
		Provider:   map[string]ProviderConfig{},
	}
	if raw == nil {
		return cfg
	}
	if v, ok := raw["default_agent"].(string); ok {
		cfg.DefaultAgent = v
	}
	if v, ok := raw["model"].(string); ok {
		cfg.Model = v
	}
	if v, ok := raw["small_model"].(string); ok {
		cfg.SmallModel = v
	}
	if v, ok := raw["username"].(string); ok {
		cfg.Username = v
	}
	if v, ok := raw["proxy"].(string); ok {
		cfg.Proxy = v
	}
	if v, ok := raw["snapshot"].(bool); ok {
		cfg.Snapshot = &v
	}
	if v, ok := raw["share"].(string); ok {
		cfg.Share = v
	}
	if v, ok := raw["instructions"]; ok {
		cfg.Instructions = stringSliceFrom(v)
	}
	if v, ok := raw["permission"].(map[string]interface{}); ok {
		cfg.Permission = v
	}
	if v, ok := raw["provider"].(map[string]interface{}); ok {
		for key, value := range v {
			entry, ok := value.(map[string]interface{})
			if !ok {
				continue
			}
			options := map[string]interface{}{}
			if opt, ok := entry["options"].(map[string]interface{}); ok {
				options = opt
			}
			cfg.Provider[key] = ProviderConfig{Options: options}
		}
	}
	applyConvenienceOptions(raw, cfg)
	if v, ok := raw["experimental"].(map[string]interface{}); ok {
		if ot, ok := v["openTelemetry"].(bool); ok {
			cfg.Experimental = &Experimental{OpenTelemetry: ot}
		}
	}
	if v, ok := raw["compaction"].(map[string]interface{}); ok {
		cfg.Compaction = decodeCompactionConfig(v)
	}
	if agentRaw, ok := raw["agent"].(map[string]interface{}); ok {
		for name, entry := range agentRaw {
			entryMap, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			cfg.Agent[name] = decodeAgentConfig(name, entryMap)
		}
	}
	if modeRaw, ok := raw["mode"].(map[string]interface{}); ok {
		for name, entry := range modeRaw {
			entryMap, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			modeCfg := decodeAgentConfig(name, entryMap)
			modeCfg.Mode = "primary"
			cfg.Agent[name] = modeCfg
		}
	}
	if commandRaw, ok := raw["command"].(map[string]interface{}); ok {
		for name, entry := range commandRaw {
			entryMap, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			cfg.Command[name] = decodeCommandConfig(name, entryMap)
		}
	}
	return cfg
}

func decodeCompactionConfig(raw map[string]interface{}) *Compaction {
	if raw == nil {
		return nil
	}
	cfg := &Compaction{}
	if v, ok := raw["auto"].(bool); ok {
		cfg.Auto = &v
	}
	if v, ok := raw["prune"].(bool); ok {
		cfg.Prune = &v
	}
	if cfg.Auto == nil && cfg.Prune == nil {
		return nil
	}
	return cfg
}

func applyConvenienceOptions(raw map[string]interface{}, cfg *Config) {
	openai := cfg.Provider["openai"]
	if openai.Options == nil {
		openai.Options = map[string]interface{}{}
	}
	if key, ok := raw["key"].(string); ok && key != "" {
		setOptionIfEmpty(openai.Options, "apiKey", key)
	}
	if base, ok := raw["baseurl"].(string); ok && base != "" {
		setOptionIfEmpty(openai.Options, "baseURL", base)
	}
	if base, ok := raw["baseURL"].(string); ok && base != "" {
		setOptionIfEmpty(openai.Options, "baseURL", base)
	}
	if proxy, ok := raw["proxy"].(string); ok && proxy != "" {
		setOptionIfEmpty(openai.Options, "proxy", proxy)
	}
	cfg.Provider["openai"] = openai
}

func setOptionIfEmpty(options map[string]interface{}, key string, value string) {
	existing, ok := options[key]
	if !ok {
		options[key] = value
		return
	}
	if text, ok := existing.(string); ok && text == "" {
		options[key] = value
	}
}

func decodeAgentConfig(name string, raw map[string]interface{}) AgentConfig {
	cfg := AgentConfig{
		Name:    name,
		Options: map[string]interface{}{},
	}
	known := map[string]bool{
		"name":        true,
		"model":       true,
		"temperature": true,
		"top_p":       true,
		"prompt":      true,
		"tools":       true,
		"disable":     true,
		"description": true,
		"mode":        true,
		"hidden":      true,
		"options":     true,
		"color":       true,
		"steps":       true,
		"maxSteps":    true,
		"permission":  true,
	}

	for key, value := range raw {
		switch key {
		case "model":
			if v, ok := value.(string); ok {
				cfg.Model = v
			}
		case "temperature":
			cfg.Temperature = floatFrom(value)
		case "top_p":
			cfg.TopP = floatFrom(value)
		case "prompt":
			if v, ok := value.(string); ok {
				cfg.Prompt = v
			}
		case "tools":
			cfg.Tools = boolMapFrom(value)
		case "disable":
			if v, ok := value.(bool); ok {
				cfg.Disable = v
			}
		case "description":
			if v, ok := value.(string); ok {
				cfg.Description = v
			}
		case "mode":
			if v, ok := value.(string); ok {
				cfg.Mode = v
			}
		case "hidden":
			if v, ok := value.(bool); ok {
				cfg.Hidden = v
			}
		case "options":
			if v, ok := value.(map[string]interface{}); ok {
				cfg.Options = v
			}
		case "color":
			if v, ok := value.(string); ok {
				cfg.Color = v
			}
		case "steps":
			cfg.Steps = intFrom(value)
		case "maxSteps":
			cfg.MaxSteps = intFrom(value)
		case "permission":
			if v, ok := value.(map[string]interface{}); ok {
				cfg.Permission = v
			}
		default:
			if !known[key] {
				cfg.Options[key] = value
			}
		}
	}

	permission := map[string]interface{}{}
	if cfg.Tools != nil {
		for tool, enabled := range cfg.Tools {
			action := "deny"
			if enabled {
				action = "allow"
			}
			if tool == "write" || tool == "edit" || tool == "patch" || tool == "multiedit" {
				permission["edit"] = action
				continue
			}
			permission[tool] = action
		}
	}
	if cfg.Permission != nil {
		permission = util.MergeMaps(permission, cfg.Permission)
	}
	cfg.Permission = permission

	if cfg.Steps == 0 && cfg.MaxSteps > 0 {
		cfg.Steps = cfg.MaxSteps
	}
	return cfg
}

func decodeCommandConfig(name string, raw map[string]interface{}) CommandConfig {
	cfg := CommandConfig{Name: name}
	for key, value := range raw {
		switch key {
		case "template":
			if v, ok := value.(string); ok {
				cfg.Template = v
			}
		case "description":
			if v, ok := value.(string); ok {
				cfg.Description = v
			}
		case "agent":
			if v, ok := value.(string); ok {
				cfg.Agent = v
			}
		case "model":
			if v, ok := value.(string); ok {
				cfg.Model = v
			}
		case "subtask":
			if v, ok := value.(bool); ok {
				cfg.Subtask = v
			}
		case "mcp":
			if v, ok := value.(bool); ok {
				cfg.MCP = v
			}
		}
	}
	return cfg
}

func floatFrom(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case json.Number:
		f, _ := v.Float64()
		return f
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}

func intFrom(value interface{}) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}

func boolMapFrom(value interface{}) map[string]bool {
	raw, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	out := map[string]bool{}
	for key, val := range raw {
		if b, ok := val.(bool); ok {
			out[key] = b
		}
	}
	return out
}

func stringSliceFrom(value interface{}) []string {
	switch v := value.(type) {
	case []interface{}:
		out := []string{}
		for _, item := range v {
			if text, ok := item.(string); ok && text != "" {
				out = append(out, text)
			}
		}
		return out
	case []string:
		return v
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	default:
		return nil
	}
}

func uniqueStrings(input []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range input {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func envBool(key string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return value == "1" || value == "true" || value == "yes"
}

func asMap(value interface{}) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}
	if m, ok := value.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

func loadWellKnownConfig(result map[string]interface{}) error {
	auths, err := auth.All()
	if err != nil {
		return err
	}
	client := http.Client{Timeout: 5 * time.Second}
	for key, info := range auths {
		if info.Type != "wellknown" {
			continue
		}
		if info.Key != "" && info.Token != "" {
			_ = os.Setenv(info.Key, info.Token)
		}
		resp, err := client.Get(key + global.WellKnownPath)
		if err != nil {
			return err
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return errors.New("failed to fetch remote config")
		}
		remote := map[string]interface{}{}
		if err := json.Unmarshal(body, &remote); err != nil {
			return err
		}
		config, ok := remote["config"].(map[string]interface{})
		if !ok {
			config = remote
		}
		result = mergeConfig(result, config)
	}
	return nil
}
