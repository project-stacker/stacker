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

# create manifest from tar
${gomtree} -K sha256digest -c -T ${t}/${name}.tar > ${t}/${name}.mtree

# check tar-manifest against the tar
${gomtree} -f ${t}/${name}.mtree -T ${t}/${name}.tar

# check tar-manifest against the filesystem
# git archive makes the uid/gid as 0, so don't check them for this test
${gomtree} -k size,sha256digest,mode,type -f ${t}/${name}.mtree -p ${t}/extract/

# create a manifest from filesystem
${gomtree} -K sha256digest -c -p ${t}/extract/ > ${t}/${name}.mtree

# check filesystem-manifest against the filesystem
${gomtree} -f ${t}/${name}.mtree -p ${t}/extract/

# check filesystem-manifest against the tar
# git archive makes the uid/gid as 0, so don't check them for this test
${gomtree} -k size,sha256digest,mode,type -f ${t}/${name}.mtree -T ${t}/${name}.tar

popd
rm -rf ${t}
