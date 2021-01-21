module github.com/anuvu/stacker

go 1.12

require (
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be
	github.com/apex/log v1.4.0
	github.com/cheggaaa/pb/v3 v3.0.4
	github.com/containers/image/v5 v5.5.1
	github.com/containers/libtrust v0.0.0-20200511145503-9c3a6c22cd9a // indirect
	github.com/containers/ocicrypt v1.0.3 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/flosch/pongo2 v0.0.0-20200529170236-5abacdfa4915 // indirect
	github.com/freddierice/go-losetup v0.0.0-20170407175016-fc9adea44124
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/klauspost/pgzip v1.2.4
	github.com/lxc/lxd v0.0.0-20200706202337-814c96fcec74
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/mitchellh/hashstructure v1.0.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.2-0.20190823105129-775207bd45b6
	github.com/opencontainers/umoci v0.4.7-0.20201029051143-b09d036cbfde
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1 // indirect
	github.com/smartystreets/assertions v1.0.1 // indirect
	github.com/smartystreets/goconvey v1.6.4
	github.com/stretchr/testify v1.6.1
	github.com/twmb/algoimpl v0.0.0-20170717182524-076353e90b94
	github.com/udhos/equalfile v0.3.0
	github.com/urfave/cli v1.22.5-0.20200826140443-af7fa3de115a
	github.com/vbatts/go-mtree v0.5.0
	github.com/vbauerster/mpb/v5 v5.2.3 // indirect
	go.etcd.io/bbolt v1.3.5 // indirect
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208 // indirect
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/lxc/go-lxc.v2 v2.0.0-20191021220249-79d3cbb2d58e
	gopkg.in/robfig/cron.v2 v2.0.0-20150107220207-be2e0b0deed5 // indirect
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/containers/image/v5 => github.com/anuvu/image/v5 v5.0.0-20200615203753-755940754545

replace github.com/freddierice/go-losetup => github.com/tych0/go-losetup v0.0.0-20200513233514-d9566aa43a61

replace github.com/opencontainers/umoci => github.com/tych0/umoci v0.4.7-0.20210121200051-7ac4c56af9ee
