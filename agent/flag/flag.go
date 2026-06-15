package flag

import (
	"os"
	"strconv"
	"strings"

	"matrixops-agent/global"
)

const DefaultOutputTokenMax = 32000

var (
	ExperimentalPlanMode      = envBool(global.EnvExperimentalPlanMode)
	ExperimentalOutputMax     = envInt(global.EnvExperimentalOutputTokenMax, DefaultOutputTokenMax)
	AutoShare                 = envBool(global.EnvAutoShare)
	Client                    = os.Getenv(global.EnvClient)
)

func envBool(name string) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return false
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
