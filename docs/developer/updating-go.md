# Updating Alloy to a new Go version

There is more to updating Go than simply updating the `go.mod` file.
You will need to submit two pull requests:

1. [Create a new build image.][build-image-instructions]
2. Update Alloy to use the new Go version, and the new build image.
   See [this][example-pr] pull request for an example.
   At this point you can just search and replace all instances of the old version with the new one.
   For example, "1.22.1" would be replaced with "1.22.5".

The Go image which is used may sometimes have a name, like "bullseye". 
The origins of the name are explained in more detail in [Go's DockerHub repository][go-dockerhub]:

> Some of these tags may have names like bookworm or bullseye in them. 
> These are the suite code names for releases of Debian⁠ and indicate which release the image is based on. 
> If your image needs to install any additional packages beyond what comes with the image, 
> you'll likely want to specify one of these explicitly to minimize breakage when there are new releases of Debian.

[build-image-instructions]:../../tools/build-image/README.md
[go-dockerhub]:https://hub.docker.com/_/golang
[example-pr]:https://github.com/grafana/alloy/pull/1256/files