#!/bin/bash
set -e

name=$(basename $0)
root="$(dirname $(dirname $(dirname $0)))"
gomtree=$(readlink -f ${root}/gomtree)
t=$(mktemp -t -d go-mtree.XXXXXX)

echo "[${name}] Running in ${t}"
# This test is for basic running check of manifest, and check against tar and file system
#

pushd ${root}

git archive --format=tar HEAD^{tree} . > ${t}/${name}.tar

prev_umask=$(umask)
umask 0 # this is so the tar command can set the mode's properly
mkdir -p ${t}/extract
tar -C ${t}/extract/ -xf ${t}/${name}.tar
umask ${prev_umask}

# create manifest from tar, ignoring non directories
${gomtree} -d -c -k type -T ${t}/${name}.tar > ${t}/${name}.mtree

# check tar-manifest against the tar
${gomtree} -d -f ${t}/${name}.mtree -T ${t}/${name}.tar

# check filesystem-manifest against the filesystem
${gomtree} -f ${t}/${name}.mtree -p ${t}/extract/

popd
rm -rf ${t}
