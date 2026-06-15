module matrixops.local/core_agent

go 1.24.1

replace pkgs => ../../pkgs

replace matrixops.local/memory => ../memory

replace matrixops-agent => ..

require pkgs v0.0.0

require matrixops.local/memory v0.0.0

require (
	matrixops-agent v0.0.0
	github.com/anthropics/anthropic-sdk-go v1.42.0
	github.com/openai/openai-go v1.12.0
	gopkg.in/yaml.v3 v3.0.1
	gorm.io/gorm v1.31.1
)

require (
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.2 // indirect
	github.com/invopop/jsonschema v0.13.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/standard-webhooks/standard-webhooks/libraries v0.0.0-20260508151727-1282bb917829 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/text v0.27.0 // indirect
)
