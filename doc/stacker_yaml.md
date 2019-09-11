## The `stacker.yaml` file

When doing a `stacker build`, the behavior of stacker is specified by the yaml
directives below. In addition to these, stacker allows variable substitions of
several forms. For example, a line like:

    $ONE ${{TWO}} ${{THREE:3}}

When run with `stacker build --substitute ONE=1 --substitute TWO=2` is
processed in stacker as:

    1 2 3

That is, variables of the form `$FOO` or `${FOO}` are supported, and variables
with `${FOO:default}` a default value will evaluate to their default if not
specified on the command line. It is an error to specify a `${FOO}` style
without a default; to make the default an empty string, use `${FOO:}`.

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

`oci`: `url` is required, of the form `path:tag`. This uses the OCI image at
`url` (which may be a local path).

`built`: `tag` is required, everything else is ignored. `built` bases this
layer on a previously specified layer in the stacker file.

`scratch`: `scratch` means a completely empty layer.

#### `import`

The `import` directive describes what files should be made available in
`/stacker` during the `run` phase. There are three forms of importing supported
today:

    /path/to/file

Will import a file or directory from the local filesystem. If the file or
directory changes between stacker builds, it will be hashed and the new file
will be imported on subsequent builds.

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

Right now there is no awareness of change for any of these bind mounts, so
--no-cache should be used to re-build if the content of the bind mount has
changed.

#### `apply`

`apply`: specifies a list of OCI/docker layers to download and apply, in skopeo
format. For example,

    apply:
        - docker://foo:latest
        - oci:oci:foo

For each entry in the list, apply will extract each layer in the image in
order, unless it has already been extracted by some other apply statement or
was part of the base layer (additionally, these layers will only be downloaded
once).

`apply` has a basic diff mechanism, so that two edits to the same file may
possibly be merged. However, if there are conflicts, apply will fail, and you
must regenerate the source layers yourself and resolve the conflicts.

#### `config`

`config` key is a special type of entry in the root in the `stacker.yaml` file.
It cannot contain a layer definition, it is used to provide configuration
applicable for building all the layers defined in this file. For example,

    config:
        prerequisites:
            - ../folder2/stacker.yaml
            - ../folder3/stacker.yaml

##### `prerequisites`
If the `prerequisites` list is present under the `config` key, stacker will
make sure to build all the layers in the stacker.yaml files found at the paths
contained in the list. This way stacker supports building multiple
stacker.yaml files in the correct order.

In this particular case the parent folder of the current folder, let's call it
`parent`, has 3 subfolders `folder1`, `folder2` and `folder3`, each containing a
`stacker.yaml` file. The example `config` above is in `parent/folder1/stacker.yaml`.

When `stacker build -f parent/folder1/stacker.yaml` is invoked, stacker would search
for the other two stacker.yaml files and build them first, before building
the stacker.yaml specified in the command line.
