module github.com/jacob-delgado/workflow

go 1.27

require (
	charm.land/bubbles/v2 v2.2.1
	charm.land/bubbletea/v2 v2.1.0
	charm.land/lipgloss/v2 v2.0.6
	charm.land/log/v2 v2.0.1
	github.com/charmbracelet/colorprofile v0.4.3
	github.com/charmbracelet/harmonica v0.2.0
	github.com/charmbracelet/x/ansi v0.11.9
	github.com/getkin/kin-openapi v0.149.0
	github.com/oapi-codegen/runtime v1.7.0
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	go.yaml.in/yaml/v3 v3.0.5
	golang.org/x/sys v0.49.0
	golang.org/x/term v0.47.0
	golang.org/x/text v0.43.0
	modernc.org/sqlite v1.60.1
)

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20261008173134-6b8d4baf91b4 // indirect; never tagged upstream: the commit charm.land/bubbletea/v2 requires, so it moves when the charm.land modules do
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/dprotaso/go-yit v0.0.0-20250513224043-18a80f8f6df4 // indirect; never tagged upstream: its last commit on gopkg.in/yaml.v3, since newer ones take go.yaml.in/yaml nodes that vmware-labs/yaml-jsonpath v0.3.2, its newest, cannot pass
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/go-logfmt/logfmt v0.6.1 // indirect
	github.com/go-openapi/jsonpointer v1.0.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mattn/go-runewidth v0.0.31 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/oapi-codegen/oapi-codegen/v2 v2.8.0 // indirect
	github.com/oasdiff/yaml v0.1.1 // indirect
	github.com/oasdiff/yaml3 v0.0.14 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	github.com/speakeasy-api/jsonpath v0.6.3 // indirect
	github.com/speakeasy-api/openapi v1.25.5 // indirect
	github.com/vmware-labs/yaml-jsonpath v0.3.2 // indirect
	github.com/xo/terminfo v1.2.0 // indirect
	golang.org/x/exp v0.0.0-20240404231335-c0f41cb1a7a0 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	modernc.org/libc v1.77.1 // indirect; held at the version modernc.org/sqlite pins, as its changelog asks of every module using it (cznic/sqlite issue 177)
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)

tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen
