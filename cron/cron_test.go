package cron

import (
	"fmt"
	"github.com/aptible/supercronic/crontab"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	TEST_CHANNEL_BUFFER_SIZE = 100
)

type testHook struct {
	channel chan *logrus.Entry
}

func newTestHook(channel chan *logrus.Entry) *testHook {
	return &testHook{channel: channel}
}

func (hook *testHook) Fire(entry *logrus.Entry) error {
	hook.channel <- entry
	return nil
}

func (hook *testHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func newTestLogger() (*logrus.Entry, chan *logrus.Entry) {
	logger := logrus.New()
	logger.Out = ioutil.Discard
	logger.Level = logrus.DebugLevel

	channel := make(chan *logrus.Entry, TEST_CHANNEL_BUFFER_SIZE)
	hook := newTestHook(channel)
	logger.Hooks.Add(hook)

	return logger.WithFields(logrus.Fields{}), channel
}

type testExpression struct {
	delay time.Duration
}

func (expr *testExpression) Next(t time.Time) time.Time {
	return t.Add(expr.delay)
}

var (
	basicContext = crontab.Context{
		Shell:   "/bin/sh",
		Environ: map[string]string{},
	}

	noData     logrus.Fields = logrus.Fields{}
	stdoutData               = logrus.Fields{"channel": "stdout"}
	stderrData               = logrus.Fields{"channel": "stderr"}
)

var runJobTestCases = []struct {
	command  string
	success  bool
	context  *crontab.Context
	messages []*logrus.Entry
}{
	{
		"true", true, &basicContext,
		[]*logrus.Entry{
			{Message: "starting", Level: logrus.InfoLevel, Data: noData},
		},
	},
	{
		"false", false, &basicContext,
		[]*logrus.Entry{
			{Message: "starting", Level: logrus.InfoLevel, Data: noData},
		},
	},
	{
		"echo hello", true, &basicContext,
		[]*logrus.Entry{
			{Message: "starting", Level: logrus.InfoLevel, Data: noData},
			{Message: "hello", Level: logrus.InfoLevel, Data: stdoutData},
		},
	},
	{
		"echo hello >&2", true, &basicContext,
		[]*logrus.Entry{
			{Message: "starting", Level: logrus.InfoLevel, Data: noData},
			{Message: "hello", Level: logrus.InfoLevel, Data: stderrData},
		},
	},
	{
		"echo $FOO", true,
		&crontab.Context{
			Shell:   "/bin/sh",
			Environ: map[string]string{"FOO": "BAR"},
		},
		[]*logrus.Entry{
			{Message: "starting", Level: logrus.InfoLevel, Data: noData},
			{Message: "BAR", Level: logrus.InfoLevel, Data: stdoutData},
		},
	},
	{
		"true", false,
		&crontab.Context{
			Shell:   "/bin/false",
			Environ: map[string]string{},
		},
		[]*logrus.Entry{
			{Message: "starting", Level: logrus.InfoLevel, Data: noData},
		},
	},
	{
		"echo hello\nsleep 0.1\necho bar >&2", true, &basicContext,
		[]*logrus.Entry{
			{Message: "starting", Level: logrus.InfoLevel, Data: noData},
			{Message: "hello", Level: logrus.InfoLevel, Data: stdoutData},
			{Message: "bar", Level: logrus.InfoLevel, Data: stderrData},
		},
	},
	{
		fmt.Sprintf("python -c 'print(\"a\" * %d * 3)'", READ_BUFFER_SIZE), true, &basicContext,
		[]*logrus.Entry{
			{Message: "starting", Level: logrus.InfoLevel, Data: noData},
			{Message: strings.Repeat("a", READ_BUFFER_SIZE), Level: logrus.InfoLevel, Data: stdoutData},
			{Message: "last line exceeded buffer size, continuing...", Level: logrus.WarnLevel, Data: stdoutData},
			{Message: strings.Repeat("a", READ_BUFFER_SIZE), Level: logrus.InfoLevel, Data: stdoutData},
			{Message: "last line exceeded buffer size, continuing...", Level: logrus.WarnLevel, Data: stdoutData},
			{Message: strings.Repeat("a", READ_BUFFER_SIZE), Level: logrus.InfoLevel, Data: stdoutData},
		},
	},
}

func TestRunJob(t *testing.T) {
	for _, tt := range runJobTestCases {
		label := fmt.Sprintf("RunJob(%q)", tt.command)
		logger, channel := newTestLogger()

		err := runJob(tt.context, tt.command, logger)
		if tt.success {
			assert.Nil(t, err, label)
		} else {
			assert.NotNil(t, err, label)
		}

		done := false

		for {
			if done || len(tt.messages) == 0 {
				break
			}

			select {
			case entry := <-channel:
				var expected *logrus.Entry
				expected, tt.messages = tt.messages[0], tt.messages[1:]
				assert.Equal(t, expected.Message, entry.Message, label)
				assert.Equal(t, expected.Level, entry.Level, label)
				assert.Equal(t, expected.Data, entry.Data, label)
			case <-time.After(time.Second):
				t.Errorf("timed out waiting for %q (%s)", tt.messages[0].Message, label)
				done = true
			}
		}
	}
}

func TestStartJobExitsOnRequest(t *testing.T) {
	job := crontab.Job{
		CrontabLine: crontab.CrontabLine{
			Expression: &testExpression{time.Minute},
			Schedule:   "always!",
			Command:    "true",
		},
		Position: 1,
	}

	exitChan := make(chan interface{}, 1)
	exitChan <- nil

	logger, _ := newTestLogger()

	var wg sync.WaitGroup

	StartJob(&wg, &basicContext, &job, exitChan, logger)

	wg.Wait()
}

func TestStartJobRunsJob(t *testing.T) {
	job := crontab.Job{
		CrontabLine: crontab.CrontabLine{
			Expression: &testExpression{2 * time.Second},
			Schedule:   "always!",
			Command:    "true",
		},
		Position: 1,
	}

	exitChan := make(chan interface{}, 1)

	var wg sync.WaitGroup

	logger, channel := newTestLogger()

	StartJob(&wg, &basicContext, &job, exitChan, logger)

	select {
	case entry := <-channel:
		assert.Regexp(t, regexp.MustCompile("job will run next"), entry.Message)
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for schedule")
	}

	select {
	case entry := <-channel:
		assert.Regexp(t, regexp.MustCompile("starting"), entry.Message)
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for start")
	}

	select {
	case entry := <-channel:
		assert.Regexp(t, regexp.MustCompile("job succeeded"), entry.Message)
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for success")
	}

	select {
	case entry := <-channel:
		assert.Regexp(t, regexp.MustCompile("job will run next"), entry.Message)
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for second schedule")
	}

	select {
	case entry := <-channel:
		assert.Regexp(t, regexp.MustCompile("starting"), entry.Message)
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for second start")
	}

	select {
	case entry := <-channel:
		assert.Regexp(t, regexp.MustCompile("job succeeded"), entry.Message)
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for second success")
	}

	exitChan <- nil
	wg.Wait()
}
