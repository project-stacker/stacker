## The `stacker.yaml` file

When doing a `stacker build`, the behavior of stacker is specified by the yaml
directives below. 

Before the yaml is parsed, stacker performs substitution on placeholders in the
file of the format `${{VAR}}` or `${{VAR:default}}`. For example, a line like:

    ${{ONE}} ${{TWO:3}}

When run with `stacker build --substitute ONE=1` is
processed in stacker as:

    1 3

In order to avoid conflict with bash or POSIX shells in the `run` section, 
only placeholders with two braces are supported, e.g. `${{FOO}}`.
Placeholders with a default value like `${{FOO:default}}` will evaluate to their default if not
specified on the command line or in a substitution file.

Using a `${{FOO}}` placeholder without a default will result in an error if
there is no substitution provided. If you want an empty string in that case, use
an empty default: `${{FOO:}}`.

In order to avoid confusion, it is also an error if a placeholder in the shell
style (`$FOO` or `${FOO}`) is found when the same key has been provided as a
substitution either via the command line (e.g. `--substitute FOO=bar`) or in a
substitution file. An error will be printed that explains how to rewrite it:

   error "A=B" was provided as a substitution and unsupported placeholder "${A}" was found. Replace "${A}" with "${{A}}" to use the substitution.

Substitutions can also be specified in a yaml file given with the argument
`--substitute-file`, with any number of key: value pairs:

    FOO: bar
    BAZ: bat

In addition to substitutions provided on the command line or a file, the
following variables are also available with their values from either command
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

### `from`

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

`oci`: `url` is required, and must be a local OCI layout URI of the form `oci:/local/path/image:tag`

`built`: `tag` is required, everything else is ignored. `built` bases this
layer on a previously specified layer in the stacker file.

`scratch`: the base image is an empty rootfs, and can be used with the `dest` field
of `import` to generate minimal images, e.g. for statically built binaries.


### `imports`

The `imports` directive describes what files should be made available in
`/stacker/imports` during the `run` phase. There are three forms of importing supported
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

Each entry in the `imports' directive also supports specifying the hash(sha256sum) of
import source, for all the three forms presented above, for example:
```
imports:
  - path: config.json
    hash: f55af805b012017bc....
  - path: http://example.com/foo.tar.gz
    hash: b458dfd63e7883a64....
  - path: stacker://$name/path/to/file
    hash: f805b012017bc769a....
```

Before copying or downloading the file, it will check if the file's hash matches
the given value. For file imports, the source file is hashed at build time. For
HTTP imports, the value returned by the server in the `X-Checksum-Sha256` HTTP
header is checked first. If that matches, the file is downloaded and then hashed
and compared again.

`stacker build` supports the flag `--require-hash`, which will cause a build
error if any http(s) remote imports do not have a hash specified, in all
transitively included stacker YAMLs.

If `--require-hash` is not passed, this import mode can be combined with unchecked imports,
and only files which have the hash specified will be checked.

```
imports:
  - path: "config.json
    hash: "BEEFcafeaaaaAAAA...."
  - /path/to/file
```

#### `import dest`

The `import` directive also supports specifying the destination path (specified
by `dest`) in the resulting container image, where the source file (specified
by `path`) will be copyied to, for example:
```
imports:
  - path: config.json
    dest: /
```


### (Deprecated) `import`
The deprecated `import` directive works like `imports` except that
the entries in the `import` array will be placed into `/stacker/` rather
than `/stacker/imports`.

See https://github.com/project-stacker/stacker/issues/571 for timeline and migration info.

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


### `environment`, `labels`, `working_dir`, `volumes`, `cmd`, `entrypoint`

These all correspond exactly to the similarly named bits in the [OCI image
config
spec](https://github.com/opencontainers/image-spec/blob/master/config.md#properties),
and are available for users to pass things through to the runtime environment
of the image.

### `runtime_user`

This sets the `user` field in the container config, as defined in the [OCI Image config spec](https://github.com/opencontainers/image-spec/blob/master/config.md#properties). 

### `generate_labels`

The `generate_labels` entry is similar to `run` in that it contains a list of
commands to run inside the generated rootfs. It runs after the `run` section is
done, and its mutations to the filesystem are not recorded, except in one case
`/oci-labels`. `/oci-labels` is a special directory where this code can write a
file, and the name of the file will be the OCI label name, and the content will
be the label content.

### `build_env` and `build_env_passthrough`

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

### `full_command`

Because of the odd behavior of `cmd` and `entrypoint` (and the inherited nature
of these from previous stacker layers), `full_command` provides a way to set
the full command that will be executed in the image, clearing out any previous
`cmd` and `entrypoint` values that were set in the image.

### `build_only`

`build_only`: indicates whether or not to include this layer in the final OCI
image. This can be useful in conjunction with an import from this layer in
another image, if you want to isolate the build environment for a binary but
not include all of its build dependencies.

### `binds`

`binds`: specifies bind mounts from the host to the container. There are three formats:

    binds:
        - /zomg
        - /foo/bar -> /bar/baz
        - source: /foo/bar
          dest: /bar/baz

The first one binds host `/zomg` to container `/zomg` while the second and third
bind host `/foo/bar` to container `/bar/baz`.

Right now there is no awareness of change for any of these bind mounts, so
--no-cache should be used to re-build if the content of the bind mount has
changed.

### `config`

`config` key is a special type of entry in the root in the `stacker.yaml` file.
It cannot contain a layer definition, it is used to provide configuration
applicable for building all the layers defined in this file. For example,

    config:
        prerequisites:
            - ../folder2/stacker.yaml
            - ../folder3/stacker.yaml

#### `prerequisites`

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

### `annotations`

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

### `os`

`os` is a user-specified string value indicating which _operating system_ this image is being
built for, for example, `linux`, `darwin`, etc. It is an optional field and it
defaults to the host operating system if not specified.

### `arch`
`arch` is a user-specified string value indicating which machine _architecture_ this image is being
built for, for example, `amd64`, `arm64`, etc. It is an optional field and it
defaults to the host machine architecture if not specified.
