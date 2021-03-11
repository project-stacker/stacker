#!/usr/bin/python3

import argparse
import glob
import multiprocessing
import os
import subprocess
import sys

storage_types=("btrfs", "overlay")
priv_levels=("priv", "unpriv")

parser = argparse.ArgumentParser()
parser.add_argument("--storage-type", choices=storage_types)
parser.add_argument("--privilege-level", choices=priv_levels)
parser.add_argument("--jobs", type=int, default=multiprocessing.cpu_count())
parser.add_argument("tests", nargs="*", default=glob.glob("./test/*.bats"))

options = parser.parse_args()

storage_to_test=storage_types
priv_to_test=priv_levels

if options.storage_type is not None:
    storage_to_test = [options.storage_type]
if options.privilege_level is not None:
    priv_to_test = [options.privilege_level]

for st in storage_to_test:
    for priv in priv_to_test:
        cmd = ["bats", "--jobs", str(options.jobs), "-t"]
        cmd.extend(options.tests)

        env = os.environ.copy()
        env["STORAGE_TYPE"] = st
        env["PRIVILEGE_LEVEL"] = priv

        print("running tests in modes:", st, priv)
        try:
            subprocess.check_call(cmd, env=env)
        except subprocess.CalledProcessError:
            print("tests in modes:", st, priv, "failed")
            sys.exit(1)
