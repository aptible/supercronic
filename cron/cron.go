package cron

import (
	"bufio"
	"fmt"
	"github.com/aptible/supercronic/crontab"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

func startReaderDrain(wg *sync.WaitGroup, readerLogger *logrus.Entry, reader io.Reader) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		scanner := bufio.NewScanner(reader)

		for scanner.Scan() {
			readerLogger.Info(scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			// The underlying reader might get closed by e.g. Wait(), or
			// even the process we're starting, so we don't log EOF-like
			// errors
			if strings.Contains(err.Error(), os.ErrClosed.Error()) {
				return
			}

			readerLogger.Error(err)
		}
	}()
}

func runJob(context *crontab.Context, command string, jobLogger *logrus.Entry) error {
	jobLogger.Info("starting")

	cmd := exec.Command(context.Shell, "-c", command)

	// Run in a separate process group so that in interactive usage, CTRL+C
	// stops supercronic, not the children threads.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	env := os.Environ()
	for k, v := range context.Environ {
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
	go startReaderDrain(&wg, stdoutLogger, stdout)

	stderrLogger := jobLogger.WithFields(logrus.Fields{"channel": "stderr"})
	go startReaderDrain(&wg, stderrLogger, stderr)

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

func StartJob(wg *sync.WaitGroup, context *crontab.Context, job *crontab.Job, exitChan chan interface{}) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		cronLogger := logrus.WithFields(logrus.Fields{
			"job.schedule": job.Schedule,
			"job.command":  job.Command, "job.position": job.Position,
		})

		var cronIteration uint64 = 0
		nextRun := job.Expression.Next(time.Now())

		// NOTE: this (intentionally) does not run multiple instances of the
		// job concurrently
		for {
			nextRun = job.Expression.Next(nextRun)
			cronLogger.Debugf("job will run next at %v", nextRun)

			delay := nextRun.Sub(time.Now())
			if delay < 0 {
				cronLogger.Warningf("job took too long to run: it should have started %v ago", -delay)
				nextRun = time.Now()
				continue
			}

			select {
			case <-exitChan:
				cronLogger.Debug("shutting down")
				return
			case <-time.After(delay):
				// Proceed normally
			}

			jobLogger := cronLogger.WithFields(logrus.Fields{
				"iteration": cronIteration,
			})

			err := runJob(context, job.Command, jobLogger)

			if err == nil {
				jobLogger.Info("job succeeded")
			} else {
				jobLogger.Error(err)
			}

			cronIteration++
		}
	}()
}
