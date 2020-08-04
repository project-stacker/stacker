## Problem Statement

Today, if many applications want to share dependencies (layers), there isn't
really a good way to do it. For example, if I have two stacker files (or
similar Docker files):

    A:
        from:
            type: docker
            url: docker://centos:latest
        run:
            - yum install openssl git
            - git clone https://example.com/A
            - ./A/install
    B:
        from:
            type: docker
            url: docker://centos:latest
        run:
            - yum install openssl git
            - git clone https://example.com/B
            - ./B/install

A and B both depend on ssl, but there is no real way to share the actual bits
that ssl depends on. There are two reasons to want to share this: 1. save disk
space, 2. know that everything image relying on ssl has all the latest security
fixes.

## Comparison With State of the Art

Today, the state of the art would be two stacker files with the same `yum
install` or `apt-get install` lines. If there is a security update, the
maintainers of A and B both need to re-build their containers in order to
deploy an up to date version.

Enter layer merging. The idea here is that we relax the constraint that a
container should be the *exact* bits it as built with, based on the insight
that most of the time the libraries are not necessarily that important, but the
application level bits are. So we keep the application level stuff bit for bit,
but allow developers to refer to libraries by a semantic name.

Then, with a suitable container runtime, application developers are not needed
to deploy security updates: instead operators just switch out the layer that
ssl points to with a new one, re-deploy the containers, and the containers see
the new versions. Development of such a runtime is left as an exercise for the
reader :)

The hardest part of this is (I think) to socialize the idea of relaxing the
*exact* bit for bit requirement.

## Implementation

The implementation stacker used to have was fairly basic. Given a stacker file with
an apply section:

    app:
        from:
            type: docker
            url: docker://ubuntu:latest
        apply:
            - docker://ssl:latest
            - docker://java:latest
        run: ...

Stacker will first download the base image, which may be made up of more than
one layer. Then, it will iterate through each of the apply entries in order (in
this case, ssl, java), and examine each layer that the tag contains but the
base does not yet contain. For each layer that is examined, it takes each file
in the layer, and compares it with the corresponding file in the base. If the
file "is equal to" the file on disk, it continues on. If the files are not
equal, stacker:

1. tests if the file is text, if not then fail
1. tries to automatically merge what is in the base with what is in the layer,
   if it can't be automatically merged, then fail

As each layer is processed, it is also inserted into the emitted OCI image. The
insight here is that the layer is included in the image. At the end of `apply`
processing, the `run` section is completed as normal, and a layer, including
the diffs from any non-identical files in the `apply` section are completed.

For example, suppose that `ubuntu:latest` contains the layers:

    e05fab2a890d758805e3f67be201e60e838e99bc -> ubuntu:latest
    39ad9e63562e5d70875918ca574d9fe0c6f550ac

And `ssl:latest` has the layers

    64fabd853e4de75a7ea3734d0a994047a1af9dc4 -> ssl:latest
    e05fab2a890d758805e3f67be201e60e838e99bc -> ubuntu:latest
    39ad9e63562e5d70875918ca574d9fe0c6f550ac

And `java:latest` has the layers

    8ab6c5e1cb34a35a35376b6c020cdf301258e428 -> java:latest
    5777ec212fc44052fd0654f808b2c67ac99aa6c3
    e05fab2a890d758805e3f67be201e60e838e99bc -> ubuntu:latest
    39ad9e63562e5d70875918ca574d9fe0c6f550ac

Suppose that the `ssl:latest` layer has no conflicts, but the `java:latest`
layer does conflict with the base. The resulting layer output will be:

    c34553482dda4a28dd22c084b2644f6cff25b2f5 -> diff from java:latest + run
    8ab6c5e1cb34a35a35376b6c020cdf301258e428 -> java:latest, included verbatim
    5777ec212fc44052fd0654f808b2c67ac99aa6c3
    64fabd853e4de75a7ea3734d0a994047a1af9dc4 -> ssl:latest, included verbatim
    e05fab2a890d758805e3f67be201e60e838e99bc -> ubuntu:latest
    39ad9e63562e5d70875918ca574d9fe0c6f550ac

## Retirement

This implementation never saw any real world use, and was standing in the way
of other innovations, so ultimately it was removed.

Additionally, it had several problems relating merging binary package databases
(rpm), or when two different layers would modify the ld.so.cache. Solving these
problems remain future work :)
