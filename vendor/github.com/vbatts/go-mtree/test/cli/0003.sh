#!/bin/bash
set -e

name=$(basename $0)
root="$(dirname $(dirname $(dirname $0)))"
gomtree=$(readlink -f ${root}/gomtree)
t=$(mktemp -t -d go-mtree.XXXXXX)

setfattr -n user.has.xattrs -v "true" "${t}" || exit 0

echo "[${name}] Running in ${t}"

mkdir "${t}/dir"
touch "${t}/dir/file"

setfattr -n user.mtree.testing -v "apples and=bananas" "${t}/dir/file"
$gomtree -c -k "sha256digest,xattrs" -p ${t}/dir > ${t}/${name}.mtree

setfattr -n user.mtree.testing -v "bananas and lemons" "${t}/dir/file"
! $gomtree -p ${t}/dir -f ${t}/${name}.mtree

setfattr -x user.mtree.testing "${t}/dir/file"
! $gomtree -p ${t}/dir -f ${t}/${name}.mtree

setfattr -n user.mtree.testing -v "apples and=bananas" "${t}/dir/file"
setfattr -n user.mtree.another -v "another  a=b" "${t}/dir/file"
! $gomtree -p ${t}/dir -f ${t}/${name}.mtree

setfattr -n user.mtree.testing -v "apples and=bananas" "${t}/dir/file"
setfattr -x user.mtree.another "${t}/dir/file"
$gomtree -p ${t}/dir -f ${t}/${name}.mtree

rm -fr ${t}
