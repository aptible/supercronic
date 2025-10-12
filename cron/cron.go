package cron

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aptible/supercronic/crontab"
	"github.com/aptible/supercronic/prometheus_metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	READ_BUFFER_SIZE = 64 * 1024
	TAIL_BUFFER_SIZE = 64 * 1024
)

// ringBuffer garde les N derniers octets écrits (tail).
type ringBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func newRingBuffer(max int) *ringBuffer {
	return &ringBuffer{max: max}
}

func (r *ringBuffer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.max <= 0 {
		return len(p), nil
	}
	if len(r.buf)+len(p) <= r.max {
		r.buf = append(r.buf, p...)
		return len(p), nil
	}
	needDrop := len(r.buf) + len(p) - r.max
	if needDrop >= len(r.buf) {
		if len(p) >= r.max {
			r.buf = append([]byte{}, p[len(p)-r.max:]...)
		} else {
			r.buf = make([]byte, 0, r.max)
			r.buf = append(r.buf, p...)
		}
		return len(p), nil
	}
	r.buf = append(r.buf[needDrop:], p...)
	return len(p), nil
}

func (r *ringBuffer) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.buf)
}

func startReaderDrain(wg *sync.WaitGroup, readerLogger *logrus.Entry, reader io.ReadCloser, tail *ringBuffer) {
	wg.Add(1)

	go func() {
		defer func() {
			if err := reader.Close(); err != nil {
				readerLogger.Errorf("failed to close pipe: %v", err)
			}
			wg.Done()
		}()

		bufReader := bufio.NewReaderSize(reader, READ_BUFFER_SIZE)

		for {
			line, isPrefix, err := bufReader.ReadLine()

			if err != nil {
				if strings.Contains(err.Error(), os.ErrClosed.Error()) {
					// The underlying reader might get
					// closed by e.g. Wait(), or even the
					// process we're starting, so we don't
					// log this.
				} else if err == io.EOF {
					// EOF, we don't need to log this
				} else {
					// Unexpected error: log it
					readerLogger.Errorf("failed to read pipe: %v", err)
				}

				break
			}

			readerLogger.Info(string(line))
			if tail != nil {
				_, _ = tail.Write(append(line, '\n'))
			}

			if isPrefix {
				readerLogger.Warn("last line exceeded buffer size, continuing...")
			}
		}
	}()
}

func handleExecError(err error) (exitCode int, wrappedErr error) {
	if err == nil {
		return 0, nil
	}

	exitCode = 1
	wrappedErr = fmt.Errorf("error running command: %v", err)

	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
			exitCode = ws.ExitStatus()
		}
	}

	return exitCode, wrappedErr
}

func runJob(
	cronCtx *crontab.Context,
	command string,
	jobLogger *logrus.Entry,
	passthroughLogs bool,
) (err error, stdoutTailStr string, stderrTailStr string, exitCode int, duration time.Duration) {

	jobLogger.Info("starting")

	cmd := exec.Command(cronCtx.Shell, "-c", command)

	// Run in a separate process group so that in interactive usage, CTRL+C
	// stops supercronic, not the children threads.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	env := os.Environ()
	for k, v := range cronCtx.Environ {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	var stdout io.ReadCloser = nil
	var stderr io.ReadCloser = nil
	exitCode = 0

	var stdoutTail, stderrTail *ringBuffer

	if passthroughLogs {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		var e error
		stdout, e = cmd.StdoutPipe()
		if e != nil {
			err = e
			return
		}
		stderr, e = cmd.StderrPipe()
		if e != nil {
			err = e
			return
		}
		stdoutTail = newRingBuffer(TAIL_BUFFER_SIZE)
		stderrTail = newRingBuffer(TAIL_BUFFER_SIZE)
	}

	start := time.Now()
	startErr := cmd.Start()
	exitCode, err = handleExecError(startErr)
	if exitCode != 0 {
		return
	}

	var wg sync.WaitGroup

	if stdout != nil {
		stdoutLogger := jobLogger.WithFields(logrus.Fields{"channel": "stdout"})
		startReaderDrain(&wg, stdoutLogger, stdout, stdoutTail)
	}

	if stderr != nil {
		stderrLogger := jobLogger.WithFields(logrus.Fields{"channel": "stderr"})
		startReaderDrain(&wg, stderrLogger, stderr, stderrTail)
	}

	wg.Wait()

	waitErr := cmd.Wait()
	duration = time.Since(start)

	if stdoutTail != nil {
		stdoutTailStr = stdoutTail.String()
	}
	if stderrTail != nil {
		stderrTailStr = stderrTail.String()
	}

	exitCode, err = handleExecError(waitErr)
	return
}

func monitorJob(ctx context.Context, job *crontab.Job, t0 time.Time, jobLogger *logrus.Entry, overlapping bool, promMetrics *prometheus_metrics.PrometheusMetrics) {
	t := t0

	for {
		t = job.Expression.Next(t)

		select {
		case <-time.After(time.Until(t)):
			m := "not starting"
			if overlapping {
				m = "overlapping jobs"
			}

			jobLogger.Warnf("%s: job is still running since %s (%s elapsed)", m, t0, t.Sub(t0))

			promMetrics.CronsDeadlineExceededCounter.With(jobPromLabels(job)).Inc()
		case <-ctx.Done():
			return
		}
	}
}

func startFunc(
	wg *sync.WaitGroup,
	exitCtx context.Context,
	logger *logrus.Entry,
	overlapping bool,
	expression crontab.Expression,
	timezone *time.Location,
	fn func(time.Time, *logrus.Entry),
) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		var jobWg sync.WaitGroup
		defer jobWg.Wait()

		var cronIteration uint64
		nextRun := time.Now().In(timezone)

		// NOTE: if overlapping is disabled (default), this does not run multiple
		// instances of the job concurrently
		for {
			nextRun = expression.Next(nextRun)
			logger.Debugf("job will run next at %v", nextRun)

			now := time.Now().In(timezone)

			delay := nextRun.Sub(now)
			if delay < 0 {
				logger.Warningf("job took too long to run: it should have started %v ago", -delay)
				nextRun = now
				continue
			}

			select {
			case <-exitCtx.Done():
				logger.Debug("shutting down")
				return
			case <-time.After(delay):
				// Proceed normally
			}

			jobWg.Add(1)

			// `nextRun` will be mutated by the next iteration of
			// this loop, so we cannot simply capture it into the
			// closure here. Instead, we make it a parameter so
			// that it gets copied when `runThisJob` is called
			runThisJob := func(cronIteration uint64, nextRun time.Time) {
				defer jobWg.Done()

				jobLogger := logger.WithFields(logrus.Fields{
					"iteration": cronIteration,
				})

				fn(nextRun, jobLogger)
			}

			if overlapping {
				go runThisJob(cronIteration, nextRun)
			} else {
				runThisJob(cronIteration, nextRun)
			}

			cronIteration++
		}
	}()
}

func StartJob(
	wg *sync.WaitGroup,
	cronCtx *crontab.Context,
	job *crontab.Job,
	exitCtx context.Context,
	cronLogger *logrus.Entry,
	overlapping bool,
	passthroughLogs bool,
	sentryExtraTrace bool,
	promMetrics *prometheus_metrics.PrometheusMetrics,
) {
	runThisJob := func(t0 time.Time, jobLogger *logrus.Entry) {
		promMetrics.CronsCurrentlyRunningGauge.With(jobPromLabels(job)).Inc()

		defer func() {
			promMetrics.CronsCurrentlyRunningGauge.With(jobPromLabels(job)).Dec()
		}()

		monitorCtx, cancelMonitor := context.WithCancel(context.Background())
		defer cancelMonitor()

		go monitorJob(monitorCtx, job, t0, jobLogger, overlapping, promMetrics)

		timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
			promMetrics.CronsExecutionTimeHistogram.With(jobPromLabels(job)).Observe(v)
		}))

		defer timer.ObserveDuration()

		err, stdout, stderr, exitCode, duration := runJob(cronCtx, job.Command, jobLogger, passthroughLogs)

		promMetrics.CronsExecCounter.With(jobPromLabels(job)).Inc()

		if err == nil {
			jobLogger.Info("job succeeded")

			promMetrics.CronsSuccessCounter.With(jobPromLabels(job)).Inc()
		} else {
			fields := logrus.Fields{
				"job.exit_code": exitCode,
				"job.duration":  duration,
			}
			if sentryExtraTrace == true {
				fields["job.stdout"] = stdout
				fields["job.stderr"] = stderr
			}

			jobLogger.WithFields(fields).Error(err)

			promMetrics.CronsFailCounter.With(jobPromLabels(job)).Inc()
		}
	}

	startFunc(
		wg,
		exitCtx,
		cronLogger,
		overlapping,
		job.Expression,
		cronCtx.Timezone,
		runThisJob,
	)
}

func jobPromLabels(job *crontab.Job) prometheus.Labels {
	return prometheus.Labels{
		"position": fmt.Sprintf("%d", job.Position),
		"command":  job.Command,
		"schedule": job.Schedule,
	}
}
