package global

const (
	AppName       = "matrixops-agent"
	ConfigDirName = ".matrixops-agent"
	WellKnownPath = "/.well-known/matrixops-agent"
)

const (
	EnvTestHome                  = "MATRIXOPS_AGENT_TEST_HOME"
	EnvConfig                    = "MATRIXOPS_AGENT_CONFIG"
	EnvConfigContent             = "MATRIXOPS_AGENT_CONFIG_CONTENT"
	EnvConfigDir                 = "MATRIXOPS_AGENT_CONFIG_DIR"
	EnvDisableProjectConfig      = "MATRIXOPS_AGENT_DISABLE_PROJECT_CONFIG"
	EnvDisableClaudeCodePrompt   = "MATRIXOPS_AGENT_DISABLE_CLAUDE_CODE_PROMPT"
	EnvPermission                = "MATRIXOPS_AGENT_PERMISSION"
	EnvModel                     = "MATRIXOPS_AGENT_MODEL"
	EnvVersion                   = "MATRIXOPS_AGENT_VERSION"
	EnvDisableModelsFetch        = "MATRIXOPS_AGENT_DISABLE_MODELS_FETCH"
	EnvExperimentalPlanMode      = "MATRIXOPS_AGENT_EXPERIMENTAL_PLAN_MODE"
	EnvExperimentalOutputTokenMax = "MATRIXOPS_AGENT_EXPERIMENTAL_OUTPUT_TOKEN_MAX"
	EnvAutoShare                 = "MATRIXOPS_AGENT_AUTO_SHARE"
	EnvClient                    = "MATRIXOPS_AGENT_CLIENT"
)

var ConfigFileNames = []string{
	AppName + ".jsonc",
	AppName + ".json",
	AppName + ".yaml",
	AppName + ".yml",
}
