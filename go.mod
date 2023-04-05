module github.com/LINBIT/virter

go 1.18

require (
	github.com/BurntSushi/toml v1.2.1
	github.com/LINBIT/containerapi v0.8.0
	github.com/LINBIT/gosshclient v0.3.1
	github.com/apparentlymart/go-cidr v1.1.0
	github.com/digitalocean/go-libvirt v0.0.0-20210615174804-eaff166426e3
	github.com/docker/go-units v0.5.0
	github.com/google/go-containerregistry v0.14.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/helm/helm v2.17.0+incompatible
	github.com/kdomanski/iso9660 v0.0.0-20200428203439-00eb28aa394d
	github.com/kr/pretty v0.3.1
	github.com/kr/text v0.2.0
	github.com/libvirt/libvirt-go-xml v7.4.0+incompatible
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/mapstructure v1.5.0
	github.com/rck/unit v0.0.3
	github.com/rodaine/table v1.1.0
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/cobra v1.7.0
	github.com/spf13/viper v1.15.0
	github.com/stretchr/testify v1.8.2
	github.com/vbauerster/mpb/v7 v7.5.3
	golang.org/x/crypto v0.7.0
	golang.org/x/sync v0.1.0
	golang.org/x/term v0.6.0
)

require (
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/alessio/shellescape v1.2.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/cli v23.0.1+incompatible // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker v23.0.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/compress v1.16.0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc2.0.20221005185240-3a7f492d3f1b // indirect
	github.com/pelletier/go-toml/v2 v2.0.6 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.9.0 // indirect
	github.com/spf13/afero v1.9.3 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/subosito/gotenv v1.4.2 // indirect
	golang.org/x/mod v0.9.0 // indirect
	golang.org/x/net v0.8.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	golang.org/x/tools v0.7.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// github.com/google/go-containerregistry@v0.6.0 has a dependency on
// github.com/containerd/containerd@v1.5.2 which, via some circular
// dependencies, depends on older versions of github.com/containerd/containerd.
// Pin the version to avoid pulling in these older versions.
replace github.com/containerd/containerd => github.com/containerd/containerd v1.5.7
