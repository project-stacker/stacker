module github.com/anuvu/stacker

go 1.12

require (
	code.cloudfoundry.org/systemcerts v0.0.0-20180917154049-ca00b2f806f2 // indirect
	github.com/14rcole/gopopulate v0.0.0-20180821133914-b175b219e774 // indirect
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/VividCortex/ewma v1.1.1 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20161002113705-648efa622239
	github.com/apex/log v1.1.0
	github.com/cheggaaa/pb v1.0.27
	github.com/containerd/continuity v0.0.0-20181203112020-004b46473808 // indirect
	github.com/containers/image v0.0.0-20190306164208-8e82e04fe1bb
	github.com/containers/storage v1.13.2 // indirect
	github.com/docker/distribution v0.0.0-20190205005809-0d3efadf0154 // indirect
	github.com/docker/docker v0.0.0-20190207111444-e6fe7f8f2936 // indirect
	github.com/docker/docker-credential-helpers v0.0.0-20180925085122-123ba1b7cd64 // indirect
	github.com/docker/go-connections v0.0.0-20180821093606-97c2040d34df // indirect
	github.com/docker/go-metrics v0.0.0-20181218153428-b84716841b82 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/etcd-io/bbolt v1.3.3 // indirect
	github.com/flosch/pongo2 v0.0.0-20181225140029-79872a7b2769 // indirect
	github.com/flynn/go-shlex v0.0.0-20150515145356-3f9db97f8568 // indirect
	github.com/freddierice/go-losetup v0.0.0-20170407175016-fc9adea44124
	github.com/ghodss/yaml v0.0.0-20190206175653-d4115522f0fe // indirect
	github.com/golang/protobuf v1.3.1 // indirect
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/gorilla/mux v1.7.0 // indirect
	github.com/gorilla/websocket v0.0.0-20190205004414-7c8e298727d1 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/juju/errors v0.0.0-20190207033735-e65537c515d7 // indirect
	github.com/klauspost/compress v1.8.1 // indirect
	github.com/klauspost/pgzip v1.2.1
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/lxc/lxd v0.0.0-20190208124523-fe0844d45b32
	github.com/mattn/go-colorable v0.1.1 // indirect
	github.com/mattn/go-isatty v0.0.7 // indirect
	github.com/mattn/go-runewidth v0.0.0-20181218000649-703b5e6b11ae // indirect
	github.com/mattn/go-shellwords v1.0.6 // indirect
	github.com/mitchellh/hashstructure v1.0.0
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/mtrmac/gpgme v0.0.0-20170102180018-b2432428689c // indirect
	github.com/openSUSE/umoci v0.4.4
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runtime-spec v1.0.1
	github.com/opencontainers/selinux v1.3.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/pquerna/ffjson v0.0.0-20190813045741-dac163c6c0a9 // indirect
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90 // indirect
	github.com/prometheus/common v0.2.0 // indirect
	github.com/prometheus/procfs v0.0.0-20190208162519-de1b801bf34b // indirect
	github.com/sergi/go-diff v0.0.0-20180205163309-da645544ed44
	github.com/stretchr/testify v1.4.0 // indirect
	github.com/twmb/algoimpl v0.0.0-20170717182524-076353e90b94
	github.com/udhos/equalfile v0.3.0
	github.com/ulikunitz/xz v0.5.5 // indirect
	github.com/urfave/cli v1.20.0
	github.com/vbatts/go-mtree v0.4.4
	github.com/vbauerster/mpb v3.4.0+incompatible // indirect
	github.com/xeipuuv/gojsonschema v1.1.0 // indirect
	go.etcd.io/bbolt v1.3.3 // indirect
	golang.org/x/crypto v0.0.0-20190829043050-9756ffdc2472 // indirect
	golang.org/x/net v0.0.0-20190827160401-ba9fcec4b297 // indirect
	golang.org/x/sync v0.0.0-20190423024810-112230192c58 // indirect
	golang.org/x/sys v0.0.0-20190830142957-1e83adbbebd0
	golang.org/x/text v0.3.2 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/cheggaaa/pb.v1 v1.0.27 // indirect
	gopkg.in/lxc/go-lxc.v2 v2.0.0-20181227225324-7c910f8a5edc
	gopkg.in/robfig/cron.v2 v2.0.0-20150107220207-be2e0b0deed5 // indirect
	gopkg.in/yaml.v2 v2.2.2
	gotest.tools v2.2.0+incompatible // indirect
	k8s.io/client-go v10.0.0+incompatible // indirect
)

replace github.com/vbatts/go-mtree v0.4.4 => github.com/vbatts/go-mtree v0.4.5-0.20190122034725-8b6de6073c1a

replace github.com/openSUSE/umoci v0.4.4 => github.com/tych0/umoci v0.1.1-0.20190402232331-556620754fb1

replace github.com/containers/image => github.com/anuvu/image v1.5.2-0.20190830191930-f48c51c45ec2
