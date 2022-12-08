package idmap

import (
	"os/user"
	"strconv"

	"github.com/lxc/lxd/shared/idmap"
	"github.com/pkg/errors"
)

func ResolveCurrentIdmapSet() (*idmap.IdmapSet, error) {
	currentUser, err := user.Current()
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't resolve current user")
	}
	return resolveIdmapSet(currentUser)
}

func resolveIdmapSet(user *user.User) (*idmap.IdmapSet, error) {
	idmapSet, err := idmap.DefaultIdmapSet("", user.Username)
	if err != nil {
		return nil, errors.Wrapf(err, "failed parsing /etc/sub{u,g}idmap")
	}

	if idmapSet != nil {
		/* Let's make our current user the root user in the ns, so that when
		 * stacker emits files, it does them as the right user.
		 */
		uid, err := strconv.Atoi(user.Uid)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't decode uid")
		}

		gid, err := strconv.Atoi(user.Gid)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't decode gid")
		}
		hostMap := []idmap.IdmapEntry{
			idmap.IdmapEntry{
				Isuid:    true,
				Hostid:   int64(uid),
				Nsid:     0,
				Maprange: 1,
			},
			idmap.IdmapEntry{
				Isgid:    true,
				Hostid:   int64(gid),
				Nsid:     0,
				Maprange: 1,
			},
		}

		for _, hm := range hostMap {
			err := idmapSet.AddSafe(hm)
			if err != nil {
				return nil, errors.Wrapf(err, "failed adding idmap entry: %v", hm)
			}
		}
	}

	return idmapSet, nil
}
