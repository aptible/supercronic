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

@test "if inotify is enabled it reloads on file change when receiving a WRITE event" {
  echo '* * * * * * * echo a > "$TEST_FILE"' > "$CRONTAB_FILE"

  "${BATS_TEST_DIRNAME}/../supercronic" -inotify "$CRONTAB_FILE" 3>&- &
  PID="$!"

  wait_for grep_test_file a

  echo '* * * * * * * echo b > "$TEST_FILE"' > "$CRONTAB_FILE"

  wait_for grep_test_file b

  kill -s TERM "$PID"
  wait
}

@test "if inotify is enabled it handles kubernetes like atomic writes using updated symlinks and folder deletion" {
  CRONTAB_FILE_NAME="$(basename $(mktemp --dry-run --tmpdir))"

  WORK_DIR="$(mktemp -d)"
  CRONTAB_PRE_DIR="$(mktemp -d)"
  CRONTAB_POST_DIR="$(mktemp -d)"

  echo '* * * * * * * echo a > "$TEST_FILE"' > "$CRONTAB_PRE_DIR"/"$CRONTAB_FILE_NAME"
  echo '* * * * * * * echo b > "$TEST_FILE"' > "$CRONTAB_POST_DIR"/"$CRONTAB_FILE_NAME"

  ln -s "$CRONTAB_PRE_DIR"/"$CRONTAB_FILE_NAME" "$WORK_DIR"/"$CRONTAB_FILE_NAME"

  "${BATS_TEST_DIRNAME}/../supercronic" -inotify -debug "$WORK_DIR"/"$CRONTAB_FILE_NAME" 3>&- &
  PID="$!"

  wait_for grep_test_file a

  ln -sf "$CRONTAB_POST_DIR"/"$CRONTAB_FILE_NAME"  "$WORK_DIR"/"$CRONTAB_FILE_NAME"

  rm -r "$CRONTAB_PRE_DIR"

  wait_for grep_test_file b

  kill -s TERM "$PID"
  wait
}
