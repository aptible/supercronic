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
	jobLineSeparator = regexp.MustCompile(`\S+`)
	envLineMatcher = regexp.MustCompile(`^([^\s=]+)\s*=\s*(.*)$`)

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

type Context struct {
	shell string
	environ map[string]string
}

type Crontab struct {
	jobs []*Job
	context *Context
}

func parseJobLine(line string) (*CrontabLine, error) {
	indices := jobLineSeparator.FindAllStringIndex(line, -1)

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

func parseCrontab(scanner *bufio.Scanner) (*Crontab, error) {
	// TODO: Don't return an array of Job, return an object representing the crontab
	// TODO: Understand environment variables, too.
	// TODO: Increment position
	position := 0

	jobs := make([]*Job, 0)

	// TODO: CRON_TZ
	environ := make(map[string]string)
	shell := "/bin/sh"

	for scanner.Scan() {
		line := strings.TrimLeft(scanner.Text(), " \t");

		if line == "" {
			continue
		}

		if line[0] == '#' {
			continue
		}

		r := envLineMatcher.FindAllStringSubmatch(line, -1)
		if len(r) == 1 && len(r[0]) == 3 {
			// TODO: Should error on setting USER?
			envKey := r[0][1]
			envVal := r[0][2]
			if envKey == "SHELL" {
				shell = envVal
			} else {
				environ[envKey] = envVal
			}
			continue
		}

		jobLine, err := parseJobLine(line)
		if (err != nil) {
			return nil, err
		}

		jobs = append(jobs, &Job{CrontabLine: *jobLine, position: position,})
		position++
	}


	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &Crontab{
		jobs: jobs,
		context: &Context{
			shell: shell,
			environ: environ,
		},
	}, nil
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

func runJob(context *Context, command string, jobLogger *logrus.Entry) error {
	jobLogger.Info("starting")

	cmd := exec.Command(context.shell, "-c", command)

	env := os.Environ()
	for k, v := range context.environ {
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

func runCron(context *Context, job *Job, exitChan chan interface{}) {
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

		if err := runJob(context, job.command, jobLogger); err != nil {
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

	crontabFileName := os.Args[1]
	logrus.Infof("read crontab: %s", crontabFileName)

	file, err := os.Open(crontabFileName)
	if err != nil {
		logrus.Fatal(err)
		return
	}
	defer file.Close()

	crontab, err := parseCrontab(bufio.NewScanner(file))

	if (err != nil) {
		logrus.Fatal(err)
		return
	}

	// TODO: Signal handling.
	// TODO: Should actually have a sync group here, and send the exit
	// request in.
	requestExitChan := make(chan interface{})

	for _, job := range crontab.jobs {
		go runCron(crontab.context, job, requestExitChan)
	}

	<-requestExitChan
}
