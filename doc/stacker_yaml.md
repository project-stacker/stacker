### The `stacker.yaml` file

The basic driver of stacker is the stackerfile, canonically named
`stacker.yaml`. The stackerfile describes a set of OCI manifests to build. For
example:

	centos:
		from:
			type: docker
			url: docker://centos:latest
		run: echo meshuggah rocks > /etc/motd

Describes a manifest which is the latest Centos image from the Docker hub.
There are various other directives which users can use to 

1. `from`: this describes the base image that stacker will start from. You can
   either start from some other image in the same stackerfile, a Docker image,
   or a tarball.
1. `import`: A set of files to download or copy into the container. Stacker
   will put these files at `/stacker`, which will be automatically cleaned up
   after the commands in the `run` section are run and the image is finalized.
   URLs (`http://example.com/file.txt`), files (`/path/to/file`), and files
   from other stacker layers (`stacker:///output/my.exe`) are all supported.
1. `run`: This is the set of commands to run in order to build the image; they
   are run in a user namespaced container, with the set of files imported
   available in `/stacker`.
1. `environment`, `labels`, `working_dir`, `volumes`, `cmd`, `entrypoint`:
   these all correspond exactly to the similarly named bits in the [OCI image
   config spec](https://github.com/opencontainers/image-spec/blob/master/config.md#properties),
   and are available for users to pass things through to the runtime environment
   of the image.
1. `full_command`: because of the odd behavior of `cmd` and `entrypoint` (and
   the inherited nature of these from previous stacker layers), `full_command`
   provides a way to set the full command that will be executed in the image,
   clearing out any previous `cmd` and `entrypoint` values that were set in the
   image.
1. `build_only`: indicates whether or not to include this layer in the final
   OCI image. This can be useful in conjunction with an import from this layer
   in another image, if you want to isolate the build environment for a binary
   but not include all of its build dependencies.

