module pkgs

go 1.25.5

require (
	matrixops-agent v0.0.0
	github.com/blevesearch/bleve/v2 v2.5.7
	github.com/mark3labs/mcp-go v0.54.0
	github.com/philippgille/chromem-go v0.7.0
	gopkg.in/yaml.v3 v3.0.1
	gorm.io/driver/sqlite v1.6.0
	gorm.io/gorm v1.31.1
)

replace matrixops-agent => ../agent

require (
	github.com/RoaringBitmap/roaring/v2 v2.4.5 // indirect
	github.com/bits-and-blooms/bitset v1.22.0 // indirect
	github.com/blevesearch/geo v0.2.4 // indirect
	github.com/blevesearch/go-faiss v1.0.26 // indirect
	github.com/blevesearch/go-porterstemmer v1.0.3 // indirect
	github.com/blevesearch/gtreap v0.1.1 // indirect
	github.com/blevesearch/mmap-go v1.0.4 // indirect
	github.com/blevesearch/scorch_segment_api/v2 v2.3.13 // indirect
	github.com/blevesearch/segment v0.9.1 // indirect
	github.com/blevesearch/snowballstem v0.9.0 // indirect
	github.com/blevesearch/upsidedown_store_api v1.0.2 // indirect
	github.com/blevesearch/vellum v1.1.0 // indirect
	github.com/blevesearch/zapx/v11 v11.4.2 // indirect
	github.com/blevesearch/zapx/v12 v12.4.2 // indirect
	github.com/blevesearch/zapx/v13 v13.4.2 // indirect
	github.com/blevesearch/zapx/v14 v14.4.2 // indirect
	github.com/blevesearch/zapx/v15 v15.4.2 // indirect
	github.com/blevesearch/zapx/v16 v16.2.8 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/json-iterator/go v0.0.0-20171115153421-f7279a603ede // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	go.etcd.io/bbolt v1.4.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)
