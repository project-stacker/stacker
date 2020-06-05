module github.com/anuvu/stacker

go 1.12

require (
	code.cloudfoundry.org/systemcerts v0.0.0-20180917154049-ca00b2f806f2 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20161002113705-648efa622239
	github.com/apex/log v1.1.1
	github.com/cheggaaa/pb v1.0.27
	github.com/containers/image v0.0.0-20190306164208-8e82e04fe1bb
	github.com/dustin/go-humanize v1.0.0
	github.com/flosch/pongo2 v0.0.0-20190707114632-bbf5a6c351f4 // indirect
	github.com/flynn/go-shlex v0.0.0-20150515145356-3f9db97f8568 // indirect
	github.com/freddierice/go-losetup v0.0.0-20170407175016-fc9adea44124
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/gorilla/websocket v0.0.0-20190205004414-7c8e298727d1 // indirect
	github.com/juju/errors v0.0.0-20190207033735-e65537c515d7 // indirect
	github.com/klauspost/pgzip v1.2.4
	github.com/lxc/lxd v0.0.0-20190208124523-fe0844d45b32
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mitchellh/hashstructure v1.0.0
	github.com/openSUSE/umoci v0.4.6-0.20200326170452-7654d6c16c17
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.2-0.20190823105129-775207bd45b6
	github.com/pkg/errors v0.9.1
	github.com/sergi/go-diff v1.0.0
	github.com/smartystreets/goconvey v1.6.4
	github.com/twmb/algoimpl v0.0.0-20170717182524-076353e90b94
	github.com/udhos/equalfile v0.3.0
	github.com/urfave/cli v1.22.1
	github.com/vbatts/go-mtree v0.5.0
	golang.org/x/crypto v0.0.0-20200423211502-4bdfaf469ed5
	golang.org/x/sys v0.0.0-20200519105757-fe76b779f299
	gopkg.in/cheggaaa/pb.v1 v1.0.27 // indirect
	gopkg.in/lxc/go-lxc.v2 v2.0.0-20181227225324-7c910f8a5edc
	gopkg.in/robfig/cron.v2 v2.0.0-20150107220207-be2e0b0deed5 // indirect
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/client-go v10.0.0+incompatible // indirect
)

replace github.com/containers/image => github.com/tych0/image v1.5.2-0.20200605140702-8ac45b805641

replace github.com/freddierice/go-losetup => github.com/tych0/go-losetup v0.0.0-20200513233514-d9566aa43a61
