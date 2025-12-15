#!/usr/bin/python3
"""
test harness for stacker
"""

import argparse
import glob
import multiprocessing
import os
import subprocess
import sys


def check_env(env_to_check):
    """
    check for required env variables
    """
    required_vars = ["ZOT_HOST", "ZOT_PORT", "REGISTRY_URL"]
    errors = []
    for req_var in required_vars:
        if req_var not in env_to_check:
            errors.append(f"missing env variable '{req_var}'")
        if not env_to_check.get(req_var):
            errors.append(f"env variable '{req_var}' is empyty")

    if len(errors) > 0:
        raise RuntimeError(f"EnvCheckFailures: {errors}")


priv_levels = ("priv", "unpriv")

parser = argparse.ArgumentParser()
parser.add_argument("--privilege-level", choices=priv_levels)
parser.add_argument("--jobs", type=int, default=multiprocessing.cpu_count())
parser.add_argument("tests", nargs="*", default=glob.glob("./test/*.bats"))

options = parser.parse_args()

priv_to_test = priv_levels

if options.privilege_level is not None:
    priv_to_test = [options.privilege_level]

for priv in priv_to_test:
    cmd = [
        "bats",
        "--setup-suite-file",
        "./test/setup_suite.bash",
        "--jobs",
        str(options.jobs),
        "--tap",
        "--timing",
    ]
    cmd.extend(options.tests)

    env = os.environ.copy()
    env["PRIVILEGE_LEVEL"] = priv
    try:
        check_env
    except RuntimeError as err:
        print(f"Failed environment variable check: {err}")
        sys.exit(1)

    print("running tests in modes:", priv)
    try:
        subprocess.check_call(cmd, env=env)
    except subprocess.CalledProcessError:
        print("tests in modes:", priv, "failed")
        sys.exit(1)
