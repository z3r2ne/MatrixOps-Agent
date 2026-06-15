module tests

go 1.24.1

require (
	matrixops.local/core_agent v0.0.0
	github.com/go-playground/assert/v2 v2.2.0
	github.com/google/uuid v1.6.0
	gorm.io/driver/sqlite v1.6.0
	gorm.io/gorm v1.31.1
)

replace matrixops.local/core_agent => ../agent/core_agent

require (
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	golang.org/x/text v0.20.0 // indirect
)
