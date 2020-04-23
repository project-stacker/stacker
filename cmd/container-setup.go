package main

import (
	"github.com/anuvu/stacker"
	"github.com/urfave/cli"
)

var containerSetupCmd = cli.Command{
	Name:   "container-setup",
	Usage:  "set up (but don't run) any containers in the stacker file",
	Hidden: true,
	Action: doContainerSetup,
	Flags:  initBuildFlags(),
	Before: beforeBuild,
	ArgsUsage: `

container-setup allows you to use stacker's OCI image handling and LXC config
generation code without actually doing a build. However, it does circumvent a
lot of the underlying cache functionality of stacker; you have been warned.

Additionally, stacker leaves its btrfs snapshots in the default (i.e.
read-only) state. If there are bind mountpoints that need to be created (or
other things liblxc or your subsequent code that runs in the container needs to
write to the filesystem), you'll want to mark them writable with:

btrfs property set -ts "$path_to_snapshot" ro false

Again, though, this circumvents stacker's awareness of its own cache, and which
may potentially screw things up.

We reserve the right to change this behavior at any time :)
`,
}

func doContainerSetup(ctx *cli.Context) error {
	args := newBuildArgs(ctx)
	args.SetupOnly = true

	builder := stacker.NewBuilder(&args)
	return builder.BuildMultiple([]string{ctx.String("stacker-file")})
}
