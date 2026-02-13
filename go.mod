module github.com/npillmayer/opentype

go 1.24.0

toolchain go1.24.11

replace github.com/go-text/typesetting => /Users/npi/prg/go/extern/typesetting

require (
	github.com/go-text/typesetting v0.0.0-00010101000000-000000000000
	github.com/go-text/typesetting-utils v0.0.0-20260203131037-09bdbf1032cb
	github.com/npillmayer/schuko v0.2.0-alpha.2
	github.com/stretchr/testify v1.7.0
	golang.org/x/image v0.34.0
)

require (
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	golang.org/x/text v0.32.0
)
