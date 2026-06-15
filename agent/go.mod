module matrixops-agent

go 1.25.5

replace matrixops.local/core_agent => ./core_agent

replace matrixops.local/memory => ./memory

replace pkgs => ../pkgs

require gorm.io/gorm v1.31.1

require (
	github.com/bmatcuk/doublestar/v4 v4.10.0 // indirect
	github.com/creack/pty v1.1.24 // indirect
	golang.org/x/net v0.34.0 // indirect
)

require (
	matrixops.local/core_agent v0.0.0
	matrixops.local/memory v0.0.0
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	golang.org/x/text v0.27.0 // indirect
	gopkg.in/yaml.v3 v3.0.1
	pkgs v0.0.0
)
