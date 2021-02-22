# Hello world example provisioning with Docker

This example contacts the VM(s) from a container and writes to the file
`/file`.

Build it:
```
docker build -t virter-hello-world:latest .
```

Use it to provision a new image:
```
virter image build ubuntu-focal hello-world-image --provision hello-world-docker.toml
```

Or use it on a running VM:
```
virter vm exec <vm_name> --provision hello-world-docker.toml
```
