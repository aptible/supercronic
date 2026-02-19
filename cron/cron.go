package cron

import (
	"bufio"
	"context"
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
)

func startReaderDrain(wg *sync.WaitGroup, readerLogger *logrus.Entry, reader io.ReadCloser) {
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

			if isPrefix {
				readerLogger.Warn("last line exceeded buffer size, continuing...")
			}
		}
	}()
}

func runJob(cronCtx *crontab.Context, command string, jobLogger *logrus.Entry, passthroughLogs bool, nextRun time.Time, replacing bool) error {
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
	var err error

	if passthroughLogs {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		stdout, err = cmd.StdoutPipe()
		if err != nil {
			return err
		}

		stderr, err = cmd.StderrPipe()
		if err != nil {
			return err
		}
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if replacing {
		ctx, cancel := context.WithDeadline(context.Background(), nextRun)
		defer cancel()
		go func(pid int) {
			// Kill command and its sub-processes once the deadline is exceeded.
			<-ctx.Done()
			if ctx.Err() == context.DeadlineExceeded {
				// Negative number tells to kill the whole process group.
				// By convention PGID of process group equals to the PID of the
				// group leader, so the command process is the first member of
				// the process group and is the group leader.
				syscall.Kill(-pid, syscall.SIGKILL)
			}
		}(cmd.Process.Pid)
	}

	var wg sync.WaitGroup

	if stdout != nil {
		stdoutLogger := jobLogger.WithFields(logrus.Fields{"channel": "stdout"})
		startReaderDrain(&wg, stdoutLogger, stdout)
	}

	if stderr != nil {
		stderrLogger := jobLogger.WithFields(logrus.Fields{"channel": "stderr"})
		startReaderDrain(&wg, stderrLogger, stderr)
	}

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error running command: %v", err)
	}

	return nil
}

func monitorJob(ctx context.Context, job *crontab.Job, t0 time.Time, jobLogger *logrus.Entry, overlapping bool, replacing bool, promMetrics *prometheus_metrics.PrometheusMetrics) {
	t := t0

	for {
		t = job.Expression.Next(t)

		select {
		case <-time.After(time.Until(t)):
			m := "not starting"
			if overlapping {
				m = "overlapping jobs"
			}
			if replacing {
				m = "replacing job"
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
	replacing bool,
	expression crontab.Expression,
	timezone *time.Location,
	fn func(time.Time, *logrus.Entry, bool),
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

				fn(nextRun, jobLogger, replacing)
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
	replacing bool,
	passthroughLogs bool,
	promMetrics *prometheus_metrics.PrometheusMetrics,
) {
	runThisJob := func(t0 time.Time, jobLogger *logrus.Entry, replacing bool) {
		promMetrics.CronsCurrentlyRunningGauge.With(jobPromLabels(job)).Inc()

		defer func() {
			promMetrics.CronsCurrentlyRunningGauge.With(jobPromLabels(job)).Dec()
		}()

		monitorCtx, cancelMonitor := context.WithCancel(context.Background())
		defer cancelMonitor()

		go monitorJob(monitorCtx, job, t0, jobLogger, overlapping, replacing, promMetrics)

		timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
			promMetrics.CronsExecutionTimeHistogram.With(jobPromLabels(job)).Observe(v)
		}))

		defer timer.ObserveDuration()

		nextRun := job.Expression.Next(t0)
		err := runJob(cronCtx, job.Command, jobLogger, passthroughLogs, nextRun, replacing)

		promMetrics.CronsExecCounter.With(jobPromLabels(job)).Inc()

		if err == nil {
			jobLogger.Info("job succeeded")

			promMetrics.CronsSuccessCounter.With(jobPromLabels(job)).Inc()
		} else {
			jobLogger.Error(err)

			promMetrics.CronsFailCounter.With(jobPromLabels(job)).Inc()
		}
	}

	startFunc(
		wg,
		exitCtx,
		cronLogger,
		overlapping,
		replacing,
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
