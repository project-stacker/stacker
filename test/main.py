#!/usr/bin/python3
"""
test harness for stacker
"""

import argparse
import glob
import multiprocessing
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile


def check_env(env_to_check):
    """
    check for required env variables
    """
    required_vars = ["ZOT_HOST", "ZOT_PORT", "REGISTRY_SERVICE", "REGISTRY_URL"]
    errors = []
    for req_var in required_vars:
        if req_var not in env_to_check:
            errors.append(f"missing env variable '{req_var}'")
        if not env_to_check.get(req_var):
            errors.append(f"env variable '{req_var}' is empyty")

    if len(errors) > 0:
        raise RuntimeError(f"EnvCheckFailures: {errors}")


def dump_log(log):
    print(f"\n--- Bats log: {log} ---")
    try:
        print(log.read_text(errors="replace"), end="")
    except OSError as err:
        print(f"Unable to read {log}: {err}")


def dump_bats_logs(tmpdir):
    run_dirs = sorted(Path(tmpdir).glob("bats-run-*"))
    failed_logs = []
    for run_dir in run_dirs:
        for stdout in (run_dir / "parallel_output").glob("*/stdout"):
            try:
                if "not ok " in stdout.read_text(errors="replace"):
                    failed_logs.append(stdout)
            except OSError:
                continue

    print(f"Bats failure logs retained in {tmpdir}")
    if failed_logs:
        for stdout in sorted(failed_logs, key=lambda log: int(log.parent.name)):
            test_name = stdout.parents[2] / "test" / f"{stdout.parent.name}.name"
            if test_name.is_file():
                print(f"\nBats failed test: {test_name.read_text().strip()}")
            dump_log(stdout)
            stderr = stdout.with_name("stderr")
            if stderr.is_file() and stderr.stat().st_size:
                dump_log(stderr)
        return

    print("No failed test worker log was found; dumping suite-level diagnostics.")
    for run_dir in run_dirs:
        for log in (run_dir / "warnings.log", run_dir / "suite.out"):
            if log.is_file() and log.stat().st_size:
                dump_log(log)


def run_bats(cmd, env):
    abort_count_warning = re.compile(
        r"# bats warning: Executed (\d+) instead of expected (\d+) tests"
    )
    run_tmpdir_notice = re.compile(r"BATS_RUN_TMPDIR: .+")
    bats = subprocess.Popen(
        cmd,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    for line in bats.stdout:
        match = abort_count_warning.fullmatch(line.rstrip("\n"))
        if match:
            print(
                "# Bats fail-fast: reported results for "
                f"{match.group(1)} of {match.group(2)} planned tests before stopping.",
                flush=True,
            )
        elif run_tmpdir_notice.fullmatch(line.rstrip("\n")):
            continue
        else:
            print(line, end="", flush=True)
    return bats.wait()


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
        # Keep test cases parallel, but let Bats report a failing file directly.
        "--no-parallelize-across-files",
        "--no-tempdir-cleanup",
        "--abort",
        "--tap",
        "--timing",
        "--verbose-run",
    ]
    cmd.extend(options.tests)

    env = os.environ.copy()
    bats_tmpdir = tempfile.mkdtemp(prefix="stacker-bats-")
    # Mode 0711 lets unprivileged tests traverse TMPDIR without listing retained logs.
    os.chmod(bats_tmpdir, 0o711)
    env["TMPDIR"] = bats_tmpdir
    env["PRIVILEGE_LEVEL"] = priv
    try:
        check_env
    except RuntimeError as err:
        print(f"Failed environment variable check: {err}")
        sys.exit(1)

    print("running tests in modes:", priv)
    try:
        if run_bats(cmd, env) != 0:
            raise subprocess.CalledProcessError(1, cmd)
    except subprocess.CalledProcessError:
        print("tests in modes:", priv, "failed")
        dump_bats_logs(bats_tmpdir)
        sys.exit(1)
    else:
        shutil.rmtree(bats_tmpdir)
