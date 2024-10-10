#!/usr/bin/env bats

function run_supercronic() {
  local crontab="$1"
  local timeout="${2:-1s}"
  timeout --preserve-status --kill-after "30s" "$timeout" \
    "${BATS_TEST_DIRNAME}/../supercronic" ${SUPERCRONIC_ARGS:-} "$crontab" 2>&1
}

setup () {
  WORK_DIR="$(mktemp -d)"
  export WORK_DIR
}

teardown() {
  rm -r "$WORK_DIR"
}

wait_for() {
  for i in $(seq 0 50); do
    if "$@" > /dev/null 2>&1; then
      return 0
    fi
    sleep 0.1
  done

  return 1
}

@test "it prints the version" {
    run "${BATS_TEST_DIRNAME}/../supercronic" -version
    [[ "$output" =~ ^v1337$ ]]
}

@test "it starts" {
  run_supercronic "${BATS_TEST_DIRNAME}/noop.crontab"
}

@test "it runs a cron job" {
  n="$(run_supercronic "${BATS_TEST_DIRNAME}/hello.crontab" 5s | grep -iE "hello from crontab.*channel=stdout" | wc -l)"
  [[ "$n" -gt 3 ]]
}

@test "it passes the environment through" {
  VAR="hello from foo" run_supercronic "${BATS_TEST_DIRNAME}/env.crontab" | grep -iE "hello from foo.*channel=stdout"
}

@test "it overrides the environment with the crontab" {
  VAR="hello from foo" run_supercronic "${BATS_TEST_DIRNAME}/override.crontab" | grep -iE "hello from bar.*channel=stdout"
}

@test "it warns when USER is set" {
  run_supercronic "${BATS_TEST_DIRNAME}/user.crontab" 1s | grep -iE "processes will not.*USER="
}

@test "it warns when a job is falling behind" {
  run_supercronic "${BATS_TEST_DIRNAME}/timeout.crontab" 1s | grep -iE "job took too long to run"
}

@test "it warns repeatedly when a job is still running" {
  n="$(run_supercronic "${BATS_TEST_DIRNAME}/timeout.crontab" 1s | grep -iE "job is still running" | wc -l)"
  [[ "$n" -eq 2 ]]
}

@test "it runs overlapping jobs" {
  n="$(SUPERCRONIC_ARGS="-overlapping" run_supercronic "${BATS_TEST_DIRNAME}/timeout.crontab" 5s | grep -iE "starting" | wc -l)"
  [[ "$n" -ge 4 ]]
}

@test "it supports debug logging " {
  SUPERCRONIC_ARGS="-debug" run_supercronic "${BATS_TEST_DIRNAME}/hello.crontab" | grep -iE "debug"
}

@test "it supports JSON logging " {
  SUPERCRONIC_ARGS="-json" run_supercronic "${BATS_TEST_DIRNAME}/noop.crontab" | grep -iE "^{"
}

@test "it supports passing command logging through" {
  SUPERCRONIC_ARGS="-passthrough-logs" run_supercronic "${BATS_TEST_DIRNAME}/hello.crontab" | grep -iE "^hello from crontab$"
}

@test "it waits for jobs to exit before terminating" {
  ready="will start"
  canary="all done"

  out="${WORK_DIR}/out"

  MSG_START="$ready" MSG_DONE="$canary" \
    "${BATS_TEST_DIRNAME}/../supercronic" "${BATS_TEST_DIRNAME}/exit.crontab" >"$out" 2>&1 &

  wait_for grep "$ready" "$out"
  kill -TERM "$!"

  wait_for grep "$canary" "$out"
}

@test "it tests a valid crontab" {
  timeout 1s "${BATS_TEST_DIRNAME}/../supercronic" -test "${BATS_TEST_DIRNAME}/noop.crontab"
}

@test "it tests an invalid crontab" {
  run timeout 1s "${BATS_TEST_DIRNAME}/../supercronic" -test "${BATS_TEST_DIRNAME}/invalid.crontab"
  [[ "$status" -eq 1 ]]
}

@test "it errors on an invalid crontab" {
  ! run_supercronic -test "${BATS_TEST_DIRNAME}/invalid.crontab"
}

@test "it run as pid 1 and reap zombie process" {
  out="${WORK_DIR}/zombie-crontab-out"

  # run in new process namespace
  sudo timeout 10s unshare --fork --pid --mount-proc \
     ${BATS_TEST_DIRNAME}/../supercronic "${BATS_TEST_DIRNAME}/zombie.crontab"  >"$out" 2>&1 &
  local pid=$!
  sleep 3

  kill -TERM ${pid}
  # todo: use other method to detect zombie cleanup
  wait_for grep "reaper cleanup: pid=" "$out"  
}


@test "it run as pid 1 and normal crontab no error" {
  out="${WORK_DIR}/normal-crontab-out"

  # sleep 30 seconds occur found bug
  # FIXME: other way to detect
  sudo timeout 30s unshare --fork --pid --mount-proc \
  "${BATS_TEST_DIRNAME}/../supercronic" "${BATS_TEST_DIRNAME}/normal.crontab" >"$out" 2>&1 &
  # https://github.com/aptible/supercronic/issues/171
  local pid=$!
  local foundErr
 
  sleep 29.5
  kill -TERM ${pid}
  grep "waitid: no child processes" "$out" && foundErr=1
  [[ $foundErr != 1 ]]
}
