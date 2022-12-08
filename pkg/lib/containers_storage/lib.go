// containers/image/storage has a dependency on libdevmapper.so; having this in
// its own package allows downstream users to import it if they want to use it,
// but means they can also avoid importing it if they don't want to add this
// dependency.
package containers_storage

import (
	"github.com/containers/image/v5/storage"
	"stackerbuild.io/stacker/pkg/lib"
)

func init() {
	lib.RegisterURLScheme("containers-storage", storage.Transport.ParseReference)
}
