#!/bin/bash

function setup_suite {
   if [ "$PRIVILEGE_LEVEL" = "priv" ]; then
      return
   fi
   if [ -z "$SUDO_UID" ]; then
      echo "setup_suite found PRIVILEGE_LEVEL=$PRIVILEGE_LEVEL but empty SUDO_USER"
      exit 1
   fi
   if [ -z "$BATS_RUN_TMPDIR" ]; then
       echo "setup_suite found empty BATS_RUN_TMPDIR"
       exit 1
   fi
   chmod ugo+x "${BATS_RUN_TMPDIR}"
}
