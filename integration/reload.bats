#!/usr/bin/env bats

setup() {
  CRONTAB_FILE="$(mktemp)"
  TEST_FILE="$(mktemp)"

  export TEST_FILE
}

teardown() {
  rm "$TEST_FILE"
}

wait_for() {
  for i in $(seq 0 5); do
    if "$@" > /dev/null 2>&1; then
      return 0
    fi

    sleep 1
  done

  return 1
}

grep_test_file() {
  grep -- "$1" "$TEST_FILE"
}

@test "it reloads on SIGUSR2" {
  echo '* * * * * * * echo a > "$TEST_FILE"' > "$CRONTAB_FILE"

  "${BATS_TEST_DIRNAME}/../supercronic" "$CRONTAB_FILE" 3>&- &
  PID="$!"

  wait_for grep_test_file a

  echo '* * * * * * * echo b > "$TEST_FILE"' > "$CRONTAB_FILE"
  kill -s USR2 "$PID"
  wait_for grep_test_file b

  kill -s TERM "$PID"
  wait
}
