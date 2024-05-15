package cron

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/aptible/supercronic/crontab"
	"github.com/aptible/supercronic/prometheus_metrics"
)

var (
	TEST_CHANNEL_BUFFER_SIZE = 100
	PROM_METRICS             = prometheus_metrics.NewPrometheusMetrics()
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
		Shell:    "/bin/sh",
		Environ:  map[string]string{},
		Timezone: time.Local,
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

		err := runJob(tt.context, tt.command, logger, false, time.Now(), false)
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
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	StartJob(&wg, &basicContext, &job, ctx, logger, false, false, false, &PROM_METRICS)

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

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	logger, channel := newTestLogger()

	StartJob(&wg, &basicContext, &job, ctx, logger, false, false, false, &PROM_METRICS)

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

	cancel()
	wg.Wait()
}

func TestStartJobReplacesPreviousJobs(t *testing.T) {
	job := crontab.Job{
		CrontabLine: crontab.CrontabLine{
			Expression: &testExpression{2 * time.Second},
			Schedule:   "always!",
			Command:    "sleep 100",
		},
		Position: 1,
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	logger, channel := newTestLogger()

	StartJob(&wg, &basicContext, &job, ctx, logger, false, true, false, &PROM_METRICS)

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
		assert.Regexp(t, regexp.MustCompile("replacing job"), entry.Message)
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for job replace warning")
	}

	select {
	case entry := <-channel:
		assert.Regexp(t, regexp.MustCompile("killed"), entry.Message)
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for job kill")
	}

	select {
	case entry := <-channel:
		assert.Regexp(t, regexp.MustCompile("job will run next"), entry.Message)
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for schedule of the second job iteration")
	}

	cancel()
	wg.Wait()
}

func TestStartFuncWaitsForCompletion(t *testing.T) {
	// We use startFunc to start a function, wait for it to start, then
	// tell the whole thing to exit, and verify that it waits for the
	// function to finish.
	expr := &testExpression{10 * time.Millisecond}

	var wg sync.WaitGroup
	logger, _ := newTestLogger()

	ctxStartFunc, cancelStartFunc := context.WithCancel(context.Background())
	ctxAllDone, allDone := context.WithCancel(context.Background())

	ctxStep1, step1Done := context.WithCancel(context.Background())
	ctxStep2, step2Done := context.WithCancel(context.Background())

	testFn := func(t0 time.Time, jobLogger *logrus.Entry, replacing bool) {
		step1Done()
		<-ctxStep2.Done()
	}

	startFunc(&wg, ctxStartFunc, logger, false, false, expr, time.Local, testFn)
	go func() {
		wg.Wait()
		allDone()
	}()

	select {
	case <-ctxStep1.Done():
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for testFn to start")
	}

	cancelStartFunc()

	select {
	case <-ctxAllDone.Done():
		t.Fatalf("wg completed before jobs finished")
	case <-time.After(time.Second):
	}

	step2Done()

	select {
	case <-ctxAllDone.Done():
	case <-time.After(time.Second):
		t.Fatalf("wg did not complete after jobs finished")
	}
}

func TestStartFuncDoesNotRunOverlappingJobs(t *testing.T) {
	// We kick off a function that does not terminate. We expect to see it
	// run only once.

	expr := &testExpression{10 * time.Millisecond}

	testChan := make(chan interface{}, TEST_CHANNEL_BUFFER_SIZE)

	var wg sync.WaitGroup
	logger, _ := newTestLogger()

	ctxStartFunc, cancelStartFunc := context.WithCancel(context.Background())
	ctxAllDone, allDone := context.WithCancel(context.Background())

	testFn := func(t0 time.Time, jobLogger *logrus.Entry, replacing bool) {
		testChan <- nil
		<-ctxAllDone.Done()
	}

	startFunc(&wg, ctxStartFunc, logger, false, false, expr, time.Local, testFn)

	select {
	case <-testChan:
	case <-time.After(time.Second):
		t.Fatalf("fn did not run")
	}

	select {
	case <-testChan:
		t.Fatalf("fn instances overlapped")
	case <-time.After(time.Second):
	}

	cancelStartFunc()
	allDone()

	wg.Wait()
}

func TestStartFuncRunsOverlappingJobs(t *testing.T) {
	// We kick off a bunch of functions that never terminate, and expect to
	// still see multiple iterations

	expr := &testExpression{10 * time.Millisecond}

	testChan := make(chan interface{}, TEST_CHANNEL_BUFFER_SIZE)

	var wg sync.WaitGroup
	logger, _ := newTestLogger()

	ctxStartFunc, cancelStartFunc := context.WithCancel(context.Background())
	ctxAllDone, allDone := context.WithCancel(context.Background())

	testFn := func(t0 time.Time, jobLogger *logrus.Entry, replacing bool) {
		testChan <- nil
		<-ctxAllDone.Done()
	}

	startFunc(&wg, ctxStartFunc, logger, true, false, expr, time.Local, testFn)

	for i := 0; i < 5; i++ {
		select {
		case <-testChan:
		case <-time.After(time.Second):
			t.Fatalf("fn instances did not overlap")
		}
	}

	cancelStartFunc()
	allDone()

	wg.Wait()
}

func TestStartFuncUsesTz(t *testing.T) {
	// Run a few instances of the cron. Check that we consistently receive
	// a time in the right TZ, which shows the time is in the right TZ
	// initially and in further iterations.
	loc := time.FixedZone("UTC+1", 1*60*60)

	expr := &testExpression{10 * time.Millisecond}

	testChan := make(chan *time.Location, TEST_CHANNEL_BUFFER_SIZE)

	var wg sync.WaitGroup
	logger, _ := newTestLogger()

	ctxStartFunc, cancelStartFunc := context.WithCancel(context.Background())

	it := 0

	testFn := func(t0 time.Time, jobLogger *logrus.Entry, replacing bool) {
		testChan <- t0.Location()
		it += 1

		if it == 1 {
			return
		}

		if it == 2 {
			// Force the next iteration to reset the iteration
			// clock
			time.Sleep(20 * time.Millisecond)
			return
		}
	}

	startFunc(&wg, ctxStartFunc, logger, false, false, expr, loc, testFn)

	for i := 0; i < 5; i++ {
		select {
		case jobLoc := <-testChan:
			assert.Equal(t, jobLoc, loc, "Timezone did not match")
		case <-time.After(time.Second):
			t.Fatalf("timeout")
		}
	}

	cancelStartFunc()
	wg.Wait()
}
