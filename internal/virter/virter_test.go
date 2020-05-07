package virter_test

//go:generate mockery -name=PortWaiter
//go:generate mockery -name=AfterNotifier
//go:generate mockery -name=NetworkCopier

const poolName = "some-pool"
const networkName = "some-network"
const imageName = "some-image"
