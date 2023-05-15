# Running Microsoft Windows with Virter

## How to create a Microsoft Windows image

This guide assumes that you have an OpenNebula instance
running. It also works without that but you must find
a way to install a standard windows OS onto a qcow2
VM Image.

For example `virt-install` can also be used to create
the virtual machine template.
[This guide](https://smig.tech/blog/tech/server_2019/)
describes how to bootstrap Windows in this way (you
still need to install cygwin and cloud init for Windows).

Note on Step 1-3: The ISO images may already be uploaded to your OpenNebula
instance. In that case, skip steps 1-3.

### Step 1: Download Server 2019 ISO

Get Windows 2019 ISO from [this location](https://www.microsoft.com/en-us/evalcenter/download-windows-server-2019)

You need to provide an eMail address and a telephone number.

### Step 2: Download virtio drivers ISO

Get virtio drivers from [this location](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/archive-virtio/virtio-win-0.1.221-1)

### Step 3: Upload to OpenNebula

Upload those images to your OpenNebula instance. This is best
done with ssh and then creating a new OpenNebula image from
a local path.

### Step 4: Create CDROM Images

In OpenNebula, create two CDROM images with Bus IDE for both images

### Step 5: Create system disk ('C:')

In OpenNebula, create an empty disk image with following settings:

 * Type Datablock
 * Persistent YES
 * Format qcow2 (under Advanced Options)
 * For bus select virtio (also under Advanced Options)
 * The size should be 200GB, but can also be less (say 40GB but I didn't try this).

### Step 6: Create a VM Template

In OpenNebula, create a VM Template with the following settings:

 * With 16GB RAM
 * With 4 Physical CPUs
 * With 8 virtual CPUs
 * With Storage: With C: Disk just created and with 2 CDROMs above
 * With Network: Office Network (with virtio drivers)
 * OS & CPU: Boot: Boot from CDROM then from C: disk
 * OS & CPU Model: Host passthru

### Step 7: Create VM Instance

In OpenNebula, create a VM instance from that new VM template

### Step 8: Install Windows

The VM instance should boot from Windows ISO

Here is a walk-though:
 * Next / Install now
 * Data Center Evaluation Desktop Experience / Next
 * I accept the license terms / Next
 * Custom: Install Windows Only
 * Load Driver .. / Ok / RedHat VirtIO Storage Driver E:\amd64\2k19\viostor.inf / Next (wait 1 minute)
 * Should show disk 0 now.
 * Next
 * Windows installs (takes about 5-10 Minutes)
 * Then on the screen set the Adminstrator password
   It should match the password used in the Cloud Init for Windows project.

### Step 9: Make mouse work

For VNC (at least for mine) the default mouse setting has to be
changed. Here is how:

 * Goto Windows Contol Panel / Hardware / Mouse / Pointer Options / Enhance Pointer Precision
 * Uncheck it and click Apply (if you can :) )

### Step 10: Allow self-signed kernel drivers

In order to run development versions of WinDRBD, we need to
allow for self signed drivers.

Here is how:

 * Open cmd (Windows-R and type cmd)
 * Then run:

    ```
    bcdedit /set TESTSIGNING ON
    ```

Changes will be made on reboot, but we will reboot in the next
step anyway.

### Step 11: Install virtio network drivers

 * Open cmd (Windows-R and type cmd)
 * Then run:

    ```
    pnputil -a E:\NetKVM\2k19\amd64\netkvm.inf
    ```

We need to reboot to make the NICs visible.
So reboot now so we can download and install things

After logging in select Yes when prompted for make this computer visible.

### Step 12: Disable automatic updates (in Server OSes only)

 * Open cmd (Windows-R and type cmd)
 * Then run:
    - sconfig<enter>
    - press 5<enter>
    - press m<enter>
    - press 15<enter> to exit sconfig

### Step 13: Install a sane browser (Firefox, Chrome)

Using Internet Explorer you need to download a modern
browser, here is how to download Firefox using Internet
explorer.

 * Go to www.mozilla.org
 * If Internet Explorer complains about a site click
    - Yes / Add / Add / Close
 * Download the Firefox installer
 * Select save, then Run
 * When starting Firefox for the first time, make firefox default browser

### Step 14: Install cygwin

We now need to install cygwin which is a bash and common
UNIX tools for Windows. Note that WSL and WSL2 do not
work, since they cannot run native Windows tools.

Here is a walk through:

 * With Firefox / Chrome go to https://www.cygwin.org/install.html
 * click and run `setup-x86_64.exe`
 * 5x Next
 * Mirror www.easyname.at (but any other should do)
 * Dropdown box on upper left: select Full
 * Select rsync and openssh in addition to defaults.
   Search for and select them. To select click Skip combo box and select
   the latest (non-testing) version.

 * Then Next / Next and wait until done (about 10-20 Minutes)
 * Click Finish when done

### Step 15: Configure sshd

Now we need to set up OpenSSH server so we can log in via
ssh. 
 * Open a CygWin terminal
 * run `ssh-host-config` with following answers:
   - strict no
   - service yes
   - CYGWIN var: binmode ntsec
 * Then run

    ```
    sc start cygsshd
    ```

 * And open port 22 using:

    ```
    netsh advfirewall firewall add rule name="Enable sshd" protocol=tcp dir=in localport=22 action=allow
    ```

  * You should be able to log into your Windows VM via ssh now.
Use ipconfig to see which IPv4 address is configured.
Log in as

    ```
    Administrator@IPv4-address
    ```

    (there is no root user on Windows)

### Step 16: Enable IPv4 pings

It is often handy to be able to ping a VM to see if it
still responds. Here is how:

 * Open a terminal (or cmd) or just login via ssh.
 * Run following command:

    ```
    netsh advfirewall firewall add rule name="Enable Pings" protocol=icmpv4:8,any dir=in action=allow
    ```

### Step 17: Install cloud-init-for-windows

`cloud-init-for-windows` is a helper script that configures
Windows VM instances for use as virter VMs. To install,
just run the install-cloud-init-0.6.exe file (or any later
version): If in an ssh session start it as follows:

```
install-cloud-init-0.6.exe /verysilent
```

You can obtain cloud init from [github](https://github.com/LINBIT/cloud-init-for-windows)

### Step 18: Shut down the VM

### Step 19: Copy the qcow2 image

Download the qcow2 image and integrate in your
CI environment (or whereever you want to use virter to run it).

Congratulations you're done!

## Tips for running virter on Microsoft Windows images

Here are some useful hints for working with virter and Windows:

  * Run with at least 4GB RAM (also on image build). Else it is just too slow.
What works is adding --vcpus 2 --memory 4G to vm run / image build commands.

  * Set `ssh_ping_count` in virter.toml file to 500.

  * When creating a VM template make sure to shut it down before pushing it, else the built-in provisioning would never be run (and vm ssh would not work).

  * Use the -u switch to configure the SSH user to Administrator (root won't work)

    ```
    -u Administrator
    ```

  * Use --vnc to configure graphical GUI access. Also add --vnc-bind-ip 0.0.0.0 if you want to access it from outside the host.

  * On image build the provisioning (bash) script must also remove the
/run/cloud-init/result.json file so the provisioning script will be
run on system start up (and configure the correct ssh host keys).

    Add:

    ```
    # make VMs configure host keys when created:
    rm /run/cloud-init/result.json
    ```

    to the provision.toml file.

  * A sample vm run command would be:

    ```
    virter vm run win2019-10 --id 50 --vcpus 2 --memory 4G -u Administrator -w --vnc --vnc-bind-ip 0.0.0.0 -l debug
    ```

  * A sample image build command would be:

    ```
    virter image build -p ../provision-ls.toml -u Administrator win2019-8 win2019-10 -l debug --vcpus 2 --memory 4G
    ```

If you have any questions direct them to johannes@johannesthoma.com

Happy hacking :)
