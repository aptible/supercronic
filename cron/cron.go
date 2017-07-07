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
	"time"
)

func drainReader(wg sync.WaitGroup, readerLogger *logrus.Entry, reader io.Reader) {
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
	go drainReader(wg, stdoutLogger, stdout)

	stderrLogger := jobLogger.WithFields(logrus.Fields{"channel": "stderr"})
	go drainReader(wg, stderrLogger, stderr)

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

func StartJob(context *crontab.Context, job *crontab.Job, exitChan chan interface{}) {
	// NOTE: this (intentionally) does not run multiple instances of the
	// job concurrently
	cronLogger := logrus.WithFields(logrus.Fields{
		"job.schedule": job.Schedule,
		"job.command":  job.Command,
		"job.position": job.Position,
	})

	var cronIteration uint64 = 0
	nextRun := job.Expression.Next(time.Now())

	for {
		nextRun = job.Expression.Next(nextRun)
		cronLogger.Debugf("job will run next at %v", nextRun)

		delay := nextRun.Sub(time.Now())
		if delay < 0 {
			cronLogger.Warningf("job took too long to run: it should have started %v ago", -delay)
			nextRun = time.Now()
			continue
		}

		time.Sleep(delay)

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
}
