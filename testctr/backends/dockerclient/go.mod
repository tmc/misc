module github.com/tmc/misc/testctr/backends/dockerclient

go 1.23.0

toolchain go1.24.3

require (
	github.com/docker/docker v28.0.1+incompatible
	github.com/docker/go-connections v0.5.0
	github.com/tmc/misc/testctr v0.0.0-00010101000000-000000000000
	github.com/tmc/misc/testctr/backends/testing/testctrbackend v0.0.0-00010101000000-000000000000
)

replace (
	github.com/tmc/misc/testctr => ../../
	github.com/tmc/misc/testctr/backends/testing/testctrbackend => ../testing/testctrbackend
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.36.0 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	gotest.tools/v3 v3.5.2 // indirect
)
