package cron

import (
	"bufio"
	"context"
	"encoding/json"
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
	// [!] Warning - these are globals, which are set in the main package
	//     Could not find a quickier and better way to pass flags from pkg main without
	//     refactoring and/or adding more params to existing functions
	JsonEnabled      = false
	ParseJsonEnabled = false
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

			// Try to parse JSON. If no valid JSON here, just log as a string
			if ParseJsonEnabled && JsonEnabled {
				err = parseJsonOrPrintText(line, readerLogger)
				if err != nil {
					// [TODO]
					readerLogger.Warn("error parsing JSON")
				}
			} else {
				readerLogger.Info(string(line))
			}

			if isPrefix {
				readerLogger.Warn("last line exceeded buffer size, continuing...")
			}
		}
	}()
}

func parseJsonOrPrintText(line []byte, readerLogger *logrus.Entry) error {
	if json.Valid(line) {
		var someJson interface{}
		err := json.Unmarshal(line, &someJson)
		if err != nil {
			return err
		}

		readerLogger.WithFields(logrus.Fields{
			"log": someJson,
		}).Info("json log data")
	} else {
		readerLogger.Info(string(line))
	}

	return nil
}

func runJob(cronCtx *crontab.Context, command string, jobLogger *logrus.Entry) error {
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

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup

	stdoutLogger := jobLogger.WithFields(logrus.Fields{"channel": "stdout"})
	startReaderDrain(&wg, stdoutLogger, stdout)

	stderrLogger := jobLogger.WithFields(logrus.Fields{"channel": "stderr"})
	startReaderDrain(&wg, stderrLogger, stderr)

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error running command: %v", err)
	}

	return nil
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

			jobLogger.Debugf("%s: job is still running since %s (%s elapsed)", m, t0, t.Sub(t0))

			promMetrics.CronsDeadlineExceededCounter.With(jobPromLabels(job)).Inc()
		case <-ctx.Done():
			return
		}
	}
}

func startFunc(wg *sync.WaitGroup, exitCtx context.Context, logger *logrus.Entry, overlapping bool, expression crontab.Expression, fn func(time.Time, *logrus.Entry)) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		var jobWg sync.WaitGroup
		defer jobWg.Wait()

		var cronIteration uint64

		nextRun := time.Now()

		// NOTE: if overlapping is disabled (default), this does t run multiple
		// instances of the job concurrently
		for {
			nextRun = expression.Next(nextRun)

			logger.Debugf("job will run next at %v", nextRun)

			delay := nextRun.Sub(time.Now())
			if delay < 0 {
				logger.Debugf("job took too long to run: it should have started %v ago", -delay)
				nextRun = time.Now()
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

			// "nextRun" param added to avoid data race with overlapping jobs
			// this could be written (to avoid addition of 2nd param) as:
			//    megaNextRun := nextRun
			// and then, in runThisJob := ... :
			//    ...
			//    fn(megaNextRun, jobLogger)
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

func StartJob(wg *sync.WaitGroup, cronCtx *crontab.Context, job *crontab.Job, exitCtx context.Context, cronLogger *logrus.Entry, overlapping bool, promMetrics *prometheus_metrics.PrometheusMetrics) {
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

		err := runJob(cronCtx, job.Command, jobLogger)

		promMetrics.CronsExecCounter.With(jobPromLabels(job)).Inc()

		if err == nil {
			jobLogger.Info("job succeeded")

			promMetrics.CronsSuccessCounter.With(jobPromLabels(job)).Inc()
		} else {
			jobLogger.Error(err)

			promMetrics.CronsFailCounter.With(jobPromLabels(job)).Inc()
		}
	}

	startFunc(wg, exitCtx, cronLogger, overlapping, job.Expression, runThisJob)
}

func jobPromLabels(job *crontab.Job) prometheus.Labels {
	return prometheus.Labels{
		"position": fmt.Sprintf("%d", job.Position),
		"command":  job.Command,
		"schedule": job.Schedule,
	}
}
