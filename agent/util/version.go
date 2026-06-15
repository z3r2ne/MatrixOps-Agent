package util

import (
	"os"

	"matrixops-agent/global"
)

const defaultVersion = "dev"

func Version() string {
	if v := os.Getenv(global.EnvVersion); v != "" {
		return v
	}
	return defaultVersion
}
