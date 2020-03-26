module github.com/anuvu/stacker

go 1.12

require (
	code.cloudfoundry.org/systemcerts v0.0.0-20180917154049-ca00b2f806f2 // indirect
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20161002113705-648efa622239
	github.com/apex/log v1.1.1
	github.com/cheggaaa/pb v1.0.27
	github.com/containerd/continuity v0.0.0-20181203112020-004b46473808 // indirect
	github.com/containers/image v0.0.0-20190306164208-8e82e04fe1bb
	github.com/docker/distribution v0.0.0-20190205005809-0d3efadf0154 // indirect
	github.com/docker/docker v0.0.0-20190207111444-e6fe7f8f2936 // indirect
	github.com/docker/go-connections v0.0.0-20180821093606-97c2040d34df // indirect
	github.com/docker/go-metrics v0.0.0-20181218153428-b84716841b82 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/flosch/pongo2 v0.0.0-20190707114632-bbf5a6c351f4 // indirect
	github.com/flynn/go-shlex v0.0.0-20150515145356-3f9db97f8568 // indirect
	github.com/freddierice/go-losetup v0.0.0-20170407175016-fc9adea44124
	github.com/ghodss/yaml v0.0.0-20190206175653-d4115522f0fe // indirect
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/gorilla/mux v1.7.0 // indirect
	github.com/gorilla/websocket v0.0.0-20190205004414-7c8e298727d1 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/juju/errors v0.0.0-20190207033735-e65537c515d7 // indirect
	github.com/klauspost/pgzip v1.2.1
	github.com/lxc/lxd v0.0.0-20190208124523-fe0844d45b32
	github.com/mattn/go-runewidth v0.0.0-20181218000649-703b5e6b11ae // indirect
	github.com/mattn/go-shellwords v1.0.6 // indirect
	github.com/mitchellh/hashstructure v1.0.0
	github.com/openSUSE/umoci v0.4.6-0.20200326170452-7654d6c16c17
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/selinux v1.3.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90 // indirect
	github.com/prometheus/common v0.2.0 // indirect
	github.com/prometheus/procfs v0.0.0-20190208162519-de1b801bf34b // indirect
	github.com/sergi/go-diff v1.0.0
	github.com/twmb/algoimpl v0.0.0-20170717182524-076353e90b94
	github.com/udhos/equalfile v0.3.0
	github.com/urfave/cli v1.22.1
	github.com/vbatts/go-mtree v0.4.4
	github.com/xeipuuv/gojsonschema v1.1.0 // indirect
	golang.org/x/sys v0.0.0-20190913121621-c3b328c6e5a7
	gopkg.in/cheggaaa/pb.v1 v1.0.27 // indirect
	gopkg.in/lxc/go-lxc.v2 v2.0.0-20181227225324-7c910f8a5edc
	gopkg.in/robfig/cron.v2 v2.0.0-20150107220207-be2e0b0deed5 // indirect
	gopkg.in/yaml.v2 v2.2.2
	gotest.tools v2.2.0+incompatible // indirect
	k8s.io/client-go v10.0.0+incompatible // indirect
)

replace github.com/vbatts/go-mtree v0.4.4 => github.com/vbatts/go-mtree v0.4.5-0.20190122034725-8b6de6073c1a

replace github.com/containers/image => github.com/anuvu/image v1.5.2-0.20190904195626-7368bc47821d
