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

@test "it supports debug logging " {
  SUPERCRONIC_ARGS="-debug" run_supercronic "${BATS_TEST_DIRNAME}/hello.crontab" | grep -iE "debug"
}

@test "it supports JSON logging " {
  SUPERCRONIC_ARGS="-json" run_supercronic "${BATS_TEST_DIRNAME}/noop.crontab" | grep -iE "^{"
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
