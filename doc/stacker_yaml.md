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

In addition to substitutions provided on the command line, the following
variables are also available with their values from either command
line flags or stacker-config file.

    STACKER_STACKER_DIR config name 'stacker_dir', cli flag '--stacker-dir'-
    STACKER_ROOTFS_DIR  config name 'rootfs_dir', cli flag '--roots-dir'
    STACKER_OCI_DIR     config name 'oci_dir', cli flag '--oci-dir'


The stacker build environment will have the following environment variables
available for reference:

  * `STACKER_LAYER_NAME`: the name of the layer being built.  `STACKER_LAYER_NAME`
    will be `my-build` when the `run` section below is executed.

      ```yaml
      my-build:
        run: echo "Your layer is ${STACKER_LAYER_NAME}"
      ```

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

#### `import hash`

The `import` directive also supports specifying the hash(sha256sum) of import source,
for all the three forms presented above, for example:
```
import:
  - path: config.json
    hash: f55af805b012017bc....
  - path: http://example.com/foo.tar.gz
    hash: b458dfd63e7883a64....
  - path: stacker://$name/path/to/file
    hash: f805b012017bc769a....
```
Before copying the file it will check if the requested hash matches the actual one.

`stacker build` supports the flag `--require-flag` which checks that all http(s) remote
imports have an hash in all stacker YAMLs.

This new import mode can be combined with the old one, for example:
```
import:
  - path: "config.json
    hash: "BEEFcafeaaaaAAAA...."
  - /path/to/file
```

### `overlay_dirs`
This directive works only with OverlayFS backend storage.

The `overlay_dirs` directive describes what directories (content) from the host should be
available in the container's filesystem. It preserves all file/dirs attributes but no
owner or group.

```
overlay_dirs:
  - source: /path/to/directory
    dest: /usr/local/          ## optional arg, default is '/'
  - source: /path/to/directory2
```
This example will result in all the files/dirs from the host's /path/to/directory 
to be available under container's /usr/local/ and all the files/dirs from the host's
/path/to/directory2 to be available under container's /


#### `environment`, `labels`, `working_dir`, `volumes`, `cmd`, `entrypoint`, `user`

These all correspond exactly to the similarly named bits in the [OCI image
config
spec](https://github.com/opencontainers/image-spec/blob/master/config.md#properties),
and are available for users to pass things through to the runtime environment
of the image.

#### `generate_labels`

The `generate_labels` entry is similar to `run` in that it contains a list of
commands to run inside the generated rootfs. It runs after the `run` section is
done, and its mutations to the filesystem are not recorded, except in one case
`/oci-labels`. `/oci-labels` is a special directory where this code can write a
file, and the name of the file will be the OCI label name, and the content will
be the label content.

#### `build_env` and `build_env_passthrough`

By default, environment variables do not pass through (pollute) the
build environment.

`build_env`: this is a dictionary with environment variable definitions.
   their values will be present in the build's environment.

`build_env_passthrough`: This is a list of regular expressions that work as a
filter on which environment variables should be passed through from the current
env into the container.  To let all variables through simply set
`build_env_passthrough`: `[".*"]`

If `build_env_passthrough` is not set, then the default value is to allow
through proxy variables `HTTP_PROXY, HTTPS_PROXY, FTP_PROXY, http_proxy,
https_proxy, ftp_proxy`.

Values in the `build_env` override values passed through via

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

##### `annotations`

`annotations` is a user-specified key value map that will be included in the
final OCI image. Note that these annotations are included in the image manifest
itself and not as part of the index.json.

    annotations:
      a.b.c.key: abc_val
      p.q.r.key: pqr_val

While `config` section supports a similar `labels`, it is more pertitent to the
image runtime. On the other hand, `annotations` is intended to be
image-specific metadata aligned with the
[annotations in the image spec](https://github.com/opencontainers/image-spec/blob/main/annotations.md).
