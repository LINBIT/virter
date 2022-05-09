module github.com/LINBIT/virter

go 1.16

require (
	github.com/BurntSushi/toml v1.1.0
	github.com/LINBIT/containerapi v0.7.0
	github.com/LINBIT/gosshclient v0.3.1
	github.com/digitalocean/go-libvirt v0.0.0-20210615174804-eaff166426e3
	github.com/docker/go-units v0.4.0
	github.com/google/go-containerregistry v0.8.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/helm/helm v2.17.0+incompatible
	github.com/kdomanski/iso9660 v0.0.0-20200428203439-00eb28aa394d
	github.com/kr/pretty v0.3.0
	github.com/kr/text v0.2.0
	github.com/libvirt/libvirt-go-xml v7.4.0+incompatible
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/mapstructure v1.5.0
	github.com/rck/unit v0.0.3
	github.com/rodaine/table v1.0.1
	github.com/sethvargo/go-signalcontext v0.1.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.3.0
	github.com/spf13/viper v1.10.0
	github.com/stretchr/testify v1.7.0
	github.com/vbauerster/mpb/v7 v7.0.2
	github.com/vektra/mockery v1.1.2
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/term v0.0.0-20210503060354-a79de5458b56
)

// github.com/google/go-containerregistry@v0.6.0 has a dependency on
// github.com/containerd/containerd@v1.5.2 which, via some circular
// dependencies, depends on older versions of github.com/containerd/containerd.
// Pin the version to avoid pulling in these older versions.
replace github.com/containerd/containerd => github.com/containerd/containerd v1.5.7
