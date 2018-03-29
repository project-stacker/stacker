# Docker Image

## Image Creation

The `Dockerfile` in this directory can be used to create a Docker image that has
`stacker` installed in it.

```
docker build . -t stacker:latest
```

## Running `stacker`

It is recommended to mount a volume to serve as your work directory in the
container. First change into the host directory that will serve as your work
directory.

```
cd <work directory>
```

Assuming that you have a stacker file `first.yaml` in your work directory, you
can create an OCI image non-interactively like this:

```
docker run --rm --privileged=true -v $(pwd):/volume -t stacker /bin/sh -c 'cd /volume && stacker build -f first.yaml'
```

You can also drop into an interactive shell inside the container like this:

```
docker run --rm --privileged=true -v $(pwd):/volume -it stacker /bin/bash

```
