module github.com/LINBIT/virter

go 1.13

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/LINBIT/containerapi v0.6.0
	github.com/LINBIT/gosshclient v0.3.1
	github.com/digitalocean/go-libvirt v0.0.0-20190715144809-7b622097a793
	github.com/docker/go-units v0.4.0
	github.com/google/go-containerregistry v0.5.1
	github.com/hashicorp/go-multierror v1.1.0
	github.com/helm/helm v2.16.6+incompatible
	github.com/kdomanski/iso9660 v0.0.0-20200428203439-00eb28aa394d
	github.com/kr/pretty v0.2.1
	github.com/kr/text v0.2.0
	github.com/libvirt/libvirt-go-xml v6.1.0+incompatible
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/mapstructure v1.4.1
	github.com/rck/unit v0.0.3
	github.com/rodaine/table v1.0.1
	github.com/sethvargo/go-signalcontext v0.1.0
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	github.com/vbauerster/mpb/v7 v7.0.2
	github.com/vektra/mockery v1.1.2
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	golang.org/x/sys v0.0.0-20210608053332-aa57babbf139 // indirect
	golang.org/x/term v0.0.0-20210503060354-a79de5458b56
)

// to enable libvirt authentication via policykit
replace github.com/digitalocean/go-libvirt v0.0.0-20190715144809-7b622097a793 => github.com/wanzenbug/go-libvirt v0.0.0-20200901120615-7281076f1c61
