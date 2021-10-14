module github.com/anuvu/stacker

go 1.16

require (
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20210929163055-e81b3f25be97 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be
	github.com/apex/log v1.9.0
	github.com/bits-and-blooms/bitset v1.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/cheggaaa/pb/v3 v3.0.8
	github.com/containerd/containerd v1.5.7 // indirect
	github.com/containers/image/v5 v5.16.0
	github.com/containers/libtrust v0.0.0-20200511145503-9c3a6c22cd9a // indirect
	github.com/containers/ocicrypt v1.1.2 // indirect
	github.com/containers/storage v1.37.0 // indirect
	github.com/docker/docker v20.10.9+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.6.4 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.13.0 // indirect
	github.com/freddierice/go-losetup v0.0.0-20210416171645-f09b6c574057
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/justincormack/go-memfd v0.0.0-20170219213707-6e4af0518993
	github.com/klauspost/pgzip v1.2.5
	github.com/lxc/go-lxc v0.0.0-20210607135324-10de240d43ab
	github.com/lxc/lxd v0.0.0-20211005075900-e00f4c9401fa
	github.com/mitchellh/hashstructure v1.1.0
	github.com/moby/term v0.0.0-20201216013528-df9cb8a40635 // indirect
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.2-0.20190823105129-775207bd45b6
	github.com/opencontainers/umoci v0.4.8-0.20210922062158-e60a0cc726e6
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.31.1 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/smartystreets/assertions v1.0.1 // indirect
	github.com/smartystreets/goconvey v1.6.4
	github.com/stretchr/testify v1.7.0
	github.com/twmb/algoimpl v0.0.0-20170717182524-076353e90b94
	github.com/udhos/equalfile v0.3.0
	github.com/urfave/cli v1.22.5
	github.com/vbatts/go-mtree v0.5.0
	github.com/vbauerster/mpb/v6 v6.0.4 // indirect
	go.mozilla.org/pkcs7 v0.0.0-20210826202110-33d05740a352 // indirect
	golang.org/x/net v0.0.0-20211005001312-d4b1ae081e3b // indirect
	golang.org/x/sys v0.0.0-20211004093028-2c5d950f24ef
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	google.golang.org/genproto v0.0.0-20211005153810-c76a74d43a8e // indirect
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/containers/image/v5 => github.com/anuvu/image/v5 v5.0.0-20210310195111-044dd755e25e

replace github.com/opencontainers/umoci => github.com/tych0/umoci v0.4.7-0.20211014154543-67f60c13162e
