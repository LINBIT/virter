package virter_test

import "github.com/LINBIT/virter/pkg/directory"

//go:generate mockery -name=ISOGenerator
//go:generate mockery -name=PortWaiter
//go:generate mockery -name=AfterNotifier
//go:generate mockery -name=NetworkCopier

func testDirectory() directory.Directory {
	return directory.Directory("../../assets/libvirt-templates")
}

const poolName = "some-pool"
const networkName = "some-network"
const imageName = "some-image"
