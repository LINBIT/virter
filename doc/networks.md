# Virter Networking

Using Virter requires at least one virtual network. All VMs started by Virter will be
added to this network, and all interactions with the guest (`vm ssh`/`vm cp`/`vm exec`/...)
happen through this network.

In almost all cases, a suitable network called `default` is already configured in libvirt.

## Defining additional networks

In addition to the access network, you can use Virter to create new virtual networks. These can
then be used to run VMs with multiple network interfaces or VMs running in multiple networks.

You can list the currently defined networks using the following command
```
$ virter network ls
Name                      Forward-Type  IP-Range          Domain  DHCP                           Bridge
default (virter default)  nat           192.168.122.1/24  test    192.168.122.2-192.168.122.254  virbr0
```

To add a second network named `net1`, configured to assign IP addresses in the range `10.255.0.0/24`, run:
```
$ virter network add net1 --dhcp --network-cidr 10.255.0.1/24
$ virter network ls
Name                      Forward-Type  IP-Range          Domain  DHCP                           Bridge
net1                                    10.255.0.1/24             10.255.0.2-10.255.0.254        virbr1
default (virter default)  nat           192.168.122.1/24  test    192.168.122.2-192.168.122.254  virbr0
```

You can also remove networks again, using `virter network rm <name>`.

## Defining VMs attached to multiple networks

You can specify additional network devices that should be added to a VM. For example, to attach a VM to the
network `net1`, run:

```
$ virter vm run alma-8 --id 8 --nic type=network,source=net1
...
$ virter network list-attached default
VM        MAC                IP             Hostname  Host Device
alma-8-8  52:54:00:00:00:08  192.168.122.8  alma-8-8  vnet1
$ virter network list-attached net1
VM        MAC                IP            Hostname  Host Device
alma-8-8  52:54:00:b3:72:d7  10.255.0.207  alma-8-8  vnet2
```

## Running VMs attached to multiple networks

Ideally, VMs started with multiple network interfaces should have all those interfaces configured as best as possible.
That means:
* All interfaces should be up.
* All interfaces running in a network with DHCP should be running a DHCP client.

Due to inconsistent behaviour of cloud-init between platforms, this is sadly not generally possible for Virter.
The limited configuration we can achieve is:
* If _all_ networks run DHCP, all those networks will be configured to use DHCP.
* If at least one network does not DHCP, cloud-init falls back to the default behaviour:
  * The first network device is configured for DHCP, which will always be the Virter access network device.
  * All other network devices are left alone. Exact behaviour depends on the guest OS. There is no guarantee
    that DHCP is configured for all networks where it is available. There is also no guarantee that the network
    interfaces are up.
