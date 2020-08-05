module github.com/czankel/cne

go 1.13

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/containerd/containerd v1.3.0
	github.com/containerd/continuity v0.0.0-20190827140505-75bee3e2ccb6 // indirect
	github.com/containerd/fifo v0.0.0-20190816180239-bda0ff6ed73c // indirect
	github.com/containerd/ttrpc v0.0.0-20191028202541-4f1b8fe65a5c // indirect
	github.com/containerd/typeurl v0.0.0-20190911142611-5eb25027c9fd // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/gogo/googleapis v1.3.0 // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/google/go-cmp v0.2.0
	github.com/google/uuid v1.1.1
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/opencontainers/runtime-spec v1.0.1
	github.com/pelletier/go-toml v1.6.0
	github.com/pkg/errors v0.8.1
	github.com/spf13/cobra v0.0.5
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2 // indirect
	golang.org/x/net v0.0.0-20191116160921-f9c825593386 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e // indirect
	golang.org/x/term v0.0.0-20191110171634-ad39bd3f0407
	google.golang.org/grpc v1.25.1 // indirect
	gopkg.in/yaml.v2 v2.2.4
)

replace github.com/docker/distribution v2.7.1+incompatible => github.com/docker/distribution v2.7.1-0.20190205005809-0d3efadf0154+incompatible
