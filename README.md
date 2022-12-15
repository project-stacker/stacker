
<p align="center">
    <img src="https://github.com/project-stacker/assets/blob/main/images/logo/stacker-logo-text.png" alt="stacker" height="130"/>
</p>


# stacker [![Build Status](https://github.com/project-stacker/stacker/workflows/ci/badge.svg?branch=master)](https://github.com/project-stacker/stacker/actions) [![Go Reference](https://pkg.go.dev/badge/stackerbuild.io/stacker.svg)](https://pkg.go.dev/stackerbuild.io/stacker)

Stacker is a tool for building OCI images natively via a declarative yaml format.

## Features

* Single binary
* Rootless builds
* Hermetically sealed builds using LXC containers
* Also available as a [GitHub action](https://github.com/project-stacker/stacker-build-push-action)

### Installation

Stacker has various [build](doc/install.md) and [runtime](doc/running.md)
dependencies.

### Hacking

See the [hacking](doc/hacking.md) guide for tips on hacking/debugging stacker.

### Usage

See the [tutorial](doc/tutorial.md) for a short introduction to how to use stacker.

See the [`stacker.yaml` specification](doc/stacker_yaml.md) for full details on
the `stacker.yaml` specification.

Additionally, there are some [tips and tricks](doc/tricks.md) for common usage.

### TODO / Roadmap

* Upstream something to containers/image that allows for automatic detection
  of compression
* Design/implement OCIv2 drafts + final spec when it comes out

### Conference Talks

* An Operator Centric Way to Update Application Containers FOSDEM 2019
    * [video](https://archive.fosdem.org/2019/schedule/event/containers_atomfs/)
    * [slides](doc/talks/FOSDEM_2019.pdf)
* Building OCI Images without Privilege OSS EU 2018
    * [slides](doc/talks/OSS_EU_2018.pdf)
* Building OCI Images without Privilege OSS NA 2018
    * [slides](doc/talks/OSS_NA_2018.pdf)

(Note that despite the similarity in name of the 2018 talks, the content is
mostly disjoint; I need to be more creative with naming.)

### License

`stacker` is released under the [Apache License, Version 2.0](LICENSE), and is:

Copyright (C) 2017-2022 Cisco Systems, Inc. and contributors
