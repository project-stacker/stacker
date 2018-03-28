## The `stacker.yaml` file

#### `from`

The `from` directive describes the base image that stacker will start from. It
takes the form:

    from:
        type: $type
        url: $url
        tag: $tag
        insecure: true

Some directives are irrelevant depending on the type. Supported types are:

`docker`: `url` is required, `insecure` is optional. When `insecure` is
specified, stacker attempts to connect via http instead of https to the Docker
Hub.

`tar`: `url` is required, everything else is ignored.

`oci`: `url` is required, `tag` is required. This uses the OCI image at `url`
(which may be a local path) and the tag `tag` as the base layer. Note: this is
not implemented currently.

`built`: `tag` is required, everything else is ignored. `built` bases this
layer on a previously specified layer in the stacker file.

`scratch`: `scratch` means a completely empty layer.

#### `import`

The `import` directive describes what files should be made available in
`/stacker` during the `run` phase. There are three forms of importing supported
today:

    /path/to/file

Will import a file from the local filesystem. If the file changes between
stacker builds, it will be hashed and the new file will be imported on
subsequent builds.

    http://example.com/foo.tar.gz

Will import foo.tar.gz and make it available in `/stacker`. Note that stacker
will NOT update this file unless the cache is cleared, to avoid excess network
usage. That means that updates after the first time stacker downloads the file
will not be reflected.

    stacker://$name/path/to/file

Will grab /path/to/file from the previously built layer `$name`.

#### `environment`, `labels, `working_dir`, `volumes`, `cmd`, `entrypoint`

These all correspond exactly to the similarly named bits in the [OCI image
config
spec](https://github.com/opencontainers/image-spec/blob/master/config.md#properties),
and are available for users to pass things through to the runtime environment
of the image.

#### `full_command`

Because of the odd behavior of `cmd` and `entrypoint` (and the inherited nature
of these from previous stacker layers), `full_command` provides a way to set
the full command that will be executed in the image, clearing out any previous
`cmd` and `entrypoint` values that were set in the image.

#### `build_only`

`build_only`: indicates whether or not to include this layer in the final OCI
image. This can be useful in conjunction with an import from this layer in
another image, if you want to isolate the build environment for a binary but
not include all of its build dependencies.

#### `binds`

`binds`: specifies bind mounts from the host to the container. There are two formats:

    binds:
        - /foo/bar -> /bar/baz
	- /zomg

The first one binds /foo/bar to /bar/baz, and the second host /zomg to
container /zomg.
