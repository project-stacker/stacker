module github.com/anuvu/stacker

go 1.12

require (
	code.cloudfoundry.org/systemcerts v0.0.0-20180917154049-ca00b2f806f2 // indirect
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/Microsoft/go-winio v0.0.0-20190117211522-75bf6ca3d7cb // indirect
	github.com/anmitsu/go-shlex v0.0.0-20161002113705-648efa622239
	github.com/apex/log v1.1.0
	github.com/boltdb/bolt v0.0.0-20180302180052-fd01fc79c553 // indirect
	github.com/cheggaaa/pb v1.0.27
	github.com/containerd/continuity v0.0.0-20181203112020-004b46473808 // indirect
	github.com/containers/image v0.0.0-20190208010805-4629bcc4825f
	github.com/containers/storage v0.0.0-20190207215558-06b6c2e4cf25 // indirect
	github.com/docker/distribution v0.0.0-20190205005809-0d3efadf0154 // indirect
	github.com/docker/docker v0.0.0-20190207111444-e6fe7f8f2936 // indirect
	github.com/docker/docker-credential-helpers v0.0.0-20180925085122-123ba1b7cd64 // indirect
	github.com/docker/go-connections v0.0.0-20180821093606-97c2040d34df // indirect
	github.com/docker/go-metrics v0.0.0-20181218153428-b84716841b82 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/flosch/pongo2 v0.0.0-20181225140029-79872a7b2769 // indirect
	github.com/flynn/go-shlex v0.0.0-20150515145356-3f9db97f8568 // indirect
	github.com/freddierice/go-losetup v0.0.0-20170407175016-fc9adea44124
	github.com/ghodss/yaml v0.0.0-20190206175653-d4115522f0fe // indirect
	github.com/golang/protobuf v1.3.1 // indirect
	github.com/google/go-cmp v0.2.0 // indirect
	github.com/gorilla/mux v1.7.0 // indirect
	github.com/gorilla/websocket v0.0.0-20190205004414-7c8e298727d1 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/juju/errors v0.0.0-20190207033735-e65537c515d7 // indirect
	github.com/klauspost/compress v1.4.1 // indirect
	github.com/klauspost/pgzip v1.2.1
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/lxc/lxd v0.0.0-20190208124523-fe0844d45b32
	github.com/mattn/go-colorable v0.1.1 // indirect
	github.com/mattn/go-isatty v0.0.7 // indirect
	github.com/mattn/go-runewidth v0.0.0-20181218000649-703b5e6b11ae // indirect
	github.com/mitchellh/hashstructure v1.0.0
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/mtrmac/gpgme v0.0.0-20170102180018-b2432428689c // indirect
	github.com/openSUSE/umoci v0.4.4
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v0.0.0-20190208075259-dd023c457d84 // indirect
	github.com/opencontainers/runtime-spec v1.0.1
	github.com/opencontainers/selinux v1.0.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90 // indirect
	github.com/prometheus/common v0.2.0 // indirect
	github.com/prometheus/procfs v0.0.0-20190208162519-de1b801bf34b // indirect
	github.com/sergi/go-diff v0.0.0-20180205163309-da645544ed44
	github.com/sirupsen/logrus v1.4.0 // indirect
	github.com/udhos/equalfile v0.3.0
	github.com/ulikunitz/xz v0.5.5 // indirect
	github.com/urfave/cli v1.20.0
	github.com/vbatts/go-mtree v0.4.4
	github.com/xeipuuv/gojsonschema v1.1.0 // indirect
	golang.org/x/crypto v0.0.0-20190325154230-a5d413f7728c // indirect
	golang.org/x/net v0.0.0-20190327091125-710a502c58a2 // indirect
	golang.org/x/sys v0.0.0-20190322080309-f49334f85ddc
	google.golang.org/genproto v0.0.0-20180831171423-11092d34479b // indirect
	gopkg.in/cheggaaa/pb.v1 v1.0.27 // indirect
	gopkg.in/lxc/go-lxc.v2 v2.0.0-20181227225324-7c910f8a5edc
	gopkg.in/robfig/cron.v2 v2.0.0-20150107220207-be2e0b0deed5 // indirect
	gopkg.in/yaml.v2 v2.2.2
	gotest.tools v2.2.0+incompatible // indirect
	k8s.io/client-go v10.0.0+incompatible // indirect
)

replace github.com/vbatts/go-mtree v0.4.4 => github.com/vbatts/go-mtree v0.4.5-0.20190122034725-8b6de6073c1a

replace github.com/openSUSE/umoci v0.4.4 => github.com/tych0/umoci v0.1.1-0.20190402232331-556620754fb1
