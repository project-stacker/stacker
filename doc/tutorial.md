## Stacker Tutorial

Stacker is a tool that allows for building OCI images in a reproducible manner,
completely unprivileged. For this tutorial, we assume you have followed the
[installation](install.md) guide and your environment satisfies all the
[runtime dependecies](running.md).

### First `stacker.yaml`

The basic input to stacker is the `stacker.yaml` file, which describes what the
base for your OCI image should be, and what to do to construct it. One of the
smallest stacker files is just:

    first:
        from:
            type: docker
            url: docker://centos:latest

Note the key `first` represents the name of the layer, and it can have any value except
`config`, which has a special usage, see the [stacker yaml](stacker_yaml.md) documentation

With this stacker file as `first.yaml`, we can do a basic stacker build:

    $ stacker build -f first.yaml
    building image first...
    importing files...
    Getting image source signatures
    Copying blob sha256:5e35d10a3ebadf9d6ab606ce72e1e77f8646b2e2ff8dd3a60d4401c3e3a76f31
     69.60 MB / 69.60 MB [=====================================================] 16s
    Copying config sha256:44a17ce607dadfb71de41d82c75d756c2bca4db677bba99969f28de726e4411e
     862 B / 862 B [============================================================] 0s
    Writing manifest to image destination
    Storing signatures
    unpacking to /home/ubuntu/tutorial/roots/_working
    running commands...
    generating layer...
    filesystem first built successfully

What happened here is that stacker downloaded the `centos:latest` tag from the
docker hub and generated it as an OCI image with tag "first". We can verify
this:

    $ umoci ls --layout oci
    centos-latest
    first

The centos-latest there is the OCI tag for the base image, and first is the
image we generated.

The next thing to note is that if we do another rebuild, less things happen:

    $ stacker build -f first.yaml
    building image first...
	importing files...
	found cached layer first

Stacker will cache all of the inputs to stacker files, and only rebuild when
one of them changes. The cache (and all of stacker's metadata) live in the `.stacker` directory where you run stacker from. Stacker's metadata can be cleaned with `stacker clean`, and its entire cache can be removed with `stacker clean --all`.

So far, the only input is a base image, but what about if we want to import a
script to run or a config file? Consider the next example:

    first:
        from:
            type: docker
            url: docker://centos:latest
        import:
            - config.json
            - install.sh
        run: |
            mkdir -p /etc/myapp
            cp /stacker/config.json /etc/myapp/
            /stacker/install.sh

If the content of `install.sh` is just `echo hello world`, then stacker's
output will look something like:

    $ stacker build -f first.yaml
	building image first...
	importing files...
	copying config.json
	copying install.sh
	Getting image source signatures
	Skipping fetch of repeat blob sha256:5e35d10a3ebadf9d6ab606ce72e1e77f8646b2e2ff8dd3a60d4401c3e3a76f31
	Copying config sha256:44a17ce607dadfb71de41d82c75d756c2bca4db677bba99969f28de726e4411e
	 862 B / 862 B [============================================================] 0s
	Writing manifest to image destination
	Storing signatures
	unpacking to /home/ubuntu/tutorial/roots/_working
	running commands...
	running commands for first
	+ mkdir -p /etc/myapp
	+ cp /stacker/config.json /etc/myapp
	+ /stacker/install.sh
	hello world
	generating layer...
	filesystem first built successfully

There are two new stacker file directives here:

    import:
        - config.json
        - install.sh

Which imports those two files into the `/stacker` directory inside the image.
This directory will not be present during the final image, so copy any files
you need out of it into their final place in the image. Also, importing things
from the web (via http://example.com/foo.tar.gz urls) is supported, and these
things will be cached on disk. Stacker will not evaluate as long as it has a
file there, so if something at the URL changes, you need to run `stacker build`
with the `--no-cache` argument, or simply delete the file from
`.stacker/imports/$target_name/foo.tar.gz`.

And then there is:

    run: |
        mkdir -p /etc/myapp
        cp /stacker/config.json /etc/myapp/
        /stacker/install.sh

Which is the set of commands to run in order to install and configure the
image.

Also note that it used a cached version of the base layer, but then re-built
the part where you asked for commands to be run, since that is new.

### dev/build containers

Finally, stacker offers "build only" containers, which are just built, but not
emitted in the final OCI image. For example:

    build:
        from:
            type: docker
            url: docker://ubuntu:latest
        run: |
            apt update
            apt install -y software-properties-common git
            apt-add-repository -y ppa:gophers/archive
            apt update
            apt install -y golang-1.9
            export PATH=$PATH:/usr/lib/go-1.9/bin
            export GOPATH=~/go
            mkdir -p $GOPATH/src/github.com/openSUSE
            cd $GOPATH/src/github.com/openSUSE
            git clone https://github.com/openSUSE/umoci
            cd umoci
            make umoci.static
            cp umoci.static /
        build_only: true
    umoci:
        from:
            type: docker
            url: docker://centos:latest
        import: stacker://build/umoci.static
        run: cp /stacker/umoci.static /usr/bin/umoci

Will build a static version of umoci in an ubuntu container, but the final
image will only contain an `umoci` tag with a statically linked version of
`umoci` at `/usr/bin/umoci`. There are a few new directives to support this:

    build_only: true

indicates that the container shouldn't be emitted in the final image, because
we're going to import something from it and don't need the rest of it. The
line:

    import: stacker://build/umoci.static

is what actually does this import, and it says "from a previously built stacker
image called 'build', import /umoci.static".
