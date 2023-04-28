#!/usr/bin/python3

import argparse
import glob
import multiprocessing
import os
import subprocess
import sys

priv_levels=("priv", "unpriv")

parser = argparse.ArgumentParser()
parser.add_argument("--privilege-level", choices=priv_levels)
parser.add_argument("--jobs", type=int, default=multiprocessing.cpu_count())
parser.add_argument("tests", nargs="*", default=glob.glob("./test/*.bats"))

options = parser.parse_args()

priv_to_test=priv_levels

if options.privilege_level is not None:
    priv_to_test = [options.privilege_level]

for priv in priv_to_test:
    cmd = ["bats", "--jobs", str(options.jobs), "--tap", "--timing"]
    cmd.extend(options.tests)

    env = os.environ.copy()
    env["PRIVILEGE_LEVEL"] = priv

    print("running tests in modes:", priv)
    try:
        subprocess.check_call(cmd, env=env)
    except subprocess.CalledProcessError:
        print("tests in modes:", priv, "failed")
        sys.exit(1)
