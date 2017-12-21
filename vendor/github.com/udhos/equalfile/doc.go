/*
Package equalfile provides facilities for comparing files.

Equalfile is similar to Python's filecmp:

        import filecmp
        if filecmp.cmp(filename1, filename2, shallow=False):

Comparing only two files

In single mode, equalfile compares files byte-by-byte.

        import "github.com/udhos/equalfile"
        // ...
       cmp := equalfile.New(nil, equalfile.Options{}) // compare using single mode
       equal, err := cmp.CompareFile("file1", "file2")

Comparing multiple files

In multiple mode, equalfile records files hashes in order to speedup repeated comparisons.
You must provide the hashing function.

        import "crypto/sha256"
        import "github.com/udhos/equalfile"
        // ...
        cmp := equalfile.NewMultiple(nil, equalfile.Options{}, sha256.New(), true) // enable multiple mode
        equal, err := cmp.CompareFile("file1", "file2")

*/
package equalfile
