module github.com/anuvu/stacker

go 1.12

require (
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be
	github.com/apex/log v1.3.0
	github.com/cheggaaa/pb/v3 v3.0.4
	github.com/containers/image/v5 v5.4.4
	github.com/containers/libtrust v0.0.0-20200511145503-9c3a6c22cd9a // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/flosch/pongo2 v0.0.0-20200529170236-5abacdfa4915 // indirect
	github.com/freddierice/go-losetup v0.0.0-20170407175016-fc9adea44124
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/klauspost/compress v1.10.8 // indirect
	github.com/klauspost/pgzip v1.2.4
	github.com/lxc/lxd v0.0.0-20200615140249-bc9af2a6adef
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/mitchellh/hashstructure v1.0.0
	github.com/openSUSE/umoci v0.4.6-0.20200326170452-7654d6c16c17
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.2-0.20190823105129-775207bd45b6
	github.com/opencontainers/runtime-spec v1.0.2 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.6.0 // indirect
	github.com/prometheus/common v0.10.0 // indirect
	github.com/prometheus/procfs v0.1.3 // indirect
	github.com/sergi/go-diff v1.1.0
	github.com/smartystreets/goconvey v1.6.4
	github.com/twmb/algoimpl v0.0.0-20170717182524-076353e90b94
	github.com/udhos/equalfile v0.3.0
	github.com/urfave/cli v1.22.4
	github.com/vbatts/go-mtree v0.5.0
	golang.org/x/crypto v0.0.0-20200604202706-70a84ac30bf9
	golang.org/x/net v0.0.0-20200602114024-627f9648deb9 // indirect
	golang.org/x/sys v0.0.0-20200615200032-f1bc736245b1
	google.golang.org/protobuf v1.24.0 // indirect
	gopkg.in/lxc/go-lxc.v2 v2.0.0-20191021220249-79d3cbb2d58e
	gopkg.in/robfig/cron.v2 v2.0.0-20150107220207-be2e0b0deed5 // indirect
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/containers/image/v5 => github.com/anuvu/image/v5 v5.0.0-20200615203753-755940754545

replace github.com/freddierice/go-losetup => github.com/tych0/go-losetup v0.0.0-20200513233514-d9566aa43a61
