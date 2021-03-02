module github.com/LINBIT/virter

go 1.13

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/LINBIT/containerapi v0.6.0
	github.com/LINBIT/gosshclient v0.3.1
	github.com/VividCortex/ewma v1.1.1 // indirect
	github.com/digitalocean/go-libvirt v0.0.0-20190715144809-7b622097a793
	github.com/hashicorp/go-multierror v1.1.0
	github.com/helm/helm v2.16.6+incompatible
	github.com/kdomanski/iso9660 v0.0.0-20200428203439-00eb28aa394d
	github.com/kr/pretty v0.2.1
	github.com/kr/text v0.2.0
	github.com/libvirt/libvirt-go-xml v6.1.0+incompatible
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/mapstructure v1.4.1
	github.com/rck/unit v0.0.3
	github.com/satori/go.uuid v1.2.0
	github.com/sethvargo/go-signalcontext v0.1.0
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	github.com/vbauerster/mpb v3.4.0+incompatible
	github.com/vektra/mockery v1.1.2
	golang.org/x/crypto v0.0.0-20201124201722-c8d3bf9c5392
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
)

// to enable libvirt authentication via policykit
replace github.com/digitalocean/go-libvirt v0.0.0-20190715144809-7b622097a793 => github.com/wanzenbug/go-libvirt v0.0.0-20200901120615-7281076f1c61
