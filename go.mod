module github.com/anuvu/stacker

go 1.16

require (
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20211102141018-f7be0cbad29c // indirect
	github.com/Microsoft/go-winio v0.5.1 // indirect
	github.com/Microsoft/hcsshim v0.9.1 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be
	github.com/apex/log v1.9.0
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/cheggaaa/pb/v3 v3.0.8
	github.com/containerd/cgroups v1.0.2 // indirect
	github.com/containerd/containerd v1.5.9 // indirect
	github.com/containers/image/v5 v5.16.1
	github.com/containers/libtrust v0.0.0-20200511145503-9c3a6c22cd9a // indirect
	github.com/docker/docker v20.10.11+incompatible // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/freddierice/go-losetup v0.0.0-20210416171645-f09b6c574057
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/justincormack/go-memfd v0.0.0-20170219213707-6e4af0518993
	github.com/klauspost/pgzip v1.2.5
	github.com/lxc/go-lxc v0.0.0-20210607135324-10de240d43ab
	github.com/lxc/lxd v0.0.0-20211118162824-0a8d8c489961
	github.com/minio/sha256-simd v1.0.0
	github.com/mitchellh/hashstructure v1.1.0
	github.com/moby/sys/mountinfo v0.5.0 // indirect
	github.com/moby/term v0.0.0-20201216013528-df9cb8a40635 // indirect
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20211202222133-eacdcc10569b
	github.com/opencontainers/runc v1.0.3 // indirect
	github.com/opencontainers/selinux v1.10.0 // indirect
	github.com/opencontainers/umoci v0.4.8-0.20211112044327-caa97eac3326
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.32.1 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/smartystreets/assertions v1.0.1 // indirect
	github.com/smartystreets/goconvey v1.6.4
	github.com/stretchr/testify v1.7.0
	github.com/twmb/algoimpl v0.0.0-20170717182524-076353e90b94
	github.com/udhos/equalfile v0.3.0
	github.com/urfave/cli v1.22.5
	github.com/vbatts/go-mtree v0.5.0
	go.mozilla.org/pkcs7 v0.0.0-20210826202110-33d05740a352 // indirect
	golang.org/x/crypto v0.0.0-20211117183948-ae814b36b871 // indirect
	golang.org/x/net v0.0.0-20220105145211-5b0dc2dfae98 // indirect
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	google.golang.org/genproto v0.0.0-20220106181925-4b6d468c965f // indirect
	google.golang.org/grpc v1.43.0 // indirect
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/containers/image/v5 => github.com/anuvu/image/v5 v5.0.0-20211117201351-4c24aa76235c
