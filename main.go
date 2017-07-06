package main

import (
	"fmt"
	"time"
	"bufio"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"strings"
	"github.com/gorhill/cronexpr"
	"github.com/sirupsen/logrus"
)

var (
	delimiter  = regexp.MustCompile(`\S+`)
	parameterCounts = []int{
		7, // POSIX + seconds + years
		6, // POSIX + years
		5, // POSIX
		1, // shorthand (e.g. @hourly)
	}
)

type CrontabLine struct {
	expression *cronexpr.Expression
	schedule string
	command  string
}

type Job struct {
	CrontabLine
	position int
}

func parseCrontabLine(line string) (*CrontabLine, error) {
	indices := delimiter.FindAllStringIndex(line, -1)

	for _, count := range parameterCounts {
		if len(indices) <= count {
			continue
		}

		scheduleEnds := indices[count - 1][1]
		commandStarts := indices[count][0]

		logrus.Debugf("try parse(%d): %s[0:%d] = %s", count, line, scheduleEnds, line[0:scheduleEnds])

		expr, err := cronexpr.Parse(line[:scheduleEnds])

		if (err != nil) {
			continue
		}

		return &CrontabLine{
			expression: expr,
			schedule: line[:scheduleEnds],
			command: line[commandStarts:],
		}, nil
	}
	return nil, fmt.Errorf("bad crontab line: %s", line)
}

func parseCrontab(scanner *bufio.Scanner) ([]*Job, error) {
	// TODO: Understand environment variables, too.
	position := 0
	ret := make([]*Job, 0)

	for scanner.Scan() {
		line := scanner.Text();

		// TODO: Allow environment variables? We may need special handling for:
		// - SHELL
		// - USER?
		parsedLine, err := parseCrontabLine(line)
		if (err != nil) {
			return nil, err
		}

		ret = append(ret, &Job{CrontabLine: *parsedLine, position: position})
	}


	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ret, nil
}

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
			if (strings.Contains(err.Error(), os.ErrClosed.Error())) {
				return
			}

			readerLogger.Error(err)
		}
	}()
}

func runJob(command string, jobLogger *logrus.Entry) error {
	jobLogger.Info("starting")

	cmd := exec.Command("/bin/sh", "-c", command)

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

	stdoutLogger := jobLogger.WithFields(logrus.Fields{ "channel": "stdout", })
	go drainReader(wg, stdoutLogger, stdout)

	stderrLogger := jobLogger.WithFields(logrus.Fields{ "channel": "stderr", })
	go drainReader(wg, stderrLogger, stderr)

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return err
	}

	jobLogger.Info("job succeeded")

	return nil
}

func runCron(job *Job, exitChan chan interface{}) {
	// NOTE: this (intentionally) does not run multiple instances of the
	// job concurrently
	cronLogger := logrus.WithFields(logrus.Fields{
		"job.schedule": job.schedule,
		"job.command": job.command,
		"job.position": job.position,
	})

	var cronIteration uint64 = 0
	nextRun := job.expression.Next(time.Now())

	for {
		nextRun = job.expression.Next(nextRun)
		cronLogger.Debugf("job will run next at %v", nextRun)

		delay := nextRun.Sub(time.Now())
		if (delay < 0) {
			cronLogger.Warningf("job took too long to run: it should have started %v ago", -delay)
			nextRun = time.Now()
			continue
		}

		time.Sleep(delay)

		jobLogger := cronLogger.WithFields(logrus.Fields{
			"iteration": cronIteration,
		})

		if err := runJob(job.command, jobLogger); err != nil {
			cronLogger.Error(err)
		}

		cronIteration++
	}
}

func main() {
	// TODO: debug flag instead
	// TODO: JSON logging?
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{ FullTimestamp: true, })

	if (len(os.Args) != 2) {
		fmt.Fprintf(os.Stderr, "Usage: %s CRONTAB\n", os.Args[0])
		os.Exit(2)
		return
	}

	crontab := os.Args[1]
	logrus.Infof("read crontab: %s", crontab)

	file, err := os.Open(crontab)
	if err != nil {
		logrus.Fatal(err)
		return
	}
	defer file.Close()

	entries, err := parseCrontab(bufio.NewScanner(file))

	if (err != nil) {
		logrus.Fatal(err)
		return
	}

	// TODO: Signal handling.
	// TODO: Should actually have a sync group here, and send the exit
	// request in.
	requestExitChan := make(chan interface{})

	for _, job := range entries {
		go runCron(job, requestExitChan)
	}

	<-requestExitChan
}
