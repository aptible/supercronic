function run_supercronic() {
  local crontab="$1"
  local timeout="${2:-2s}"
  timeout --preserve-status --kill-after "30s" "$timeout" "${BATS_TEST_DIRNAME}/../supercronic" "$crontab" 2>&1
}

@test "it runs a cron job" {
  run_supercronic "${BATS_TEST_DIRNAME}/hello.crontab" | grep -iE "hello from crontab.*channel=stdout"
}

@test "it passes the environment through" {
  VAR="hello from foo" run_supercronic "${BATS_TEST_DIRNAME}/env.crontab" | grep -iE "hello from foo.*channel=stdout"
}

@test "it overrides the environment with the crontab" {
  VAR="hello from foo" run_supercronic "${BATS_TEST_DIRNAME}/override.crontab" | grep -iE "hello from bar.*channel=stdout"
}

@test "it warns when USER is set" {
  run_supercronic "${BATS_TEST_DIRNAME}/user.crontab" 5s | grep -iE "processes will not.*USER="
}

@test "it warns when a job is falling behind" {
  run_supercronic "${BATS_TEST_DIRNAME}/timeout.crontab" 5s | grep -iE "job took too long to run"
}
