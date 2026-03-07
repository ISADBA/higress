module github.com/alibaba/higress/plugins/wasm-go/extensions/scope-priority-test

go 1.24.1

toolchain go1.24.4

replace github.com/higress-group/wasm-go => github.com/ISADBA/wasm-go v1.0.9-consumer-support-v2

require (
	github.com/higress-group/proxy-wasm-go-sdk v0.0.0-20251103120604-77e9cce339d2
	github.com/higress-group/wasm-go v1.0.2-0.20250821081215-b573359becf8
	github.com/tidwall/gjson v1.18.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/resp v0.1.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
)
