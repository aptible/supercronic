package crontab

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/gorhill/cronexpr"
	"github.com/sirupsen/logrus"
)

var (
	jobLineSeparator = regexp.MustCompile(`\S+`)
	envLineMatcher   = regexp.MustCompile(`^([^\s=]+)\s*=\s*(.*)$`)

	parameterCounts = []int{
		7, // POSIX + seconds + years
		6, // POSIX + years
		5, // POSIX
		1, // shorthand (e.g. @hourly)
	}
)

func parseJobLine(line string) (*CrontabLine, error) {
	indices := jobLineSeparator.FindAllStringIndex(line, -1)

	for _, count := range parameterCounts {
		if len(indices) <= count {
			continue
		}

		scheduleEnds := indices[count-1][1]
		commandStarts := indices[count][0]

		// TODO: Should receive a logger?
		logrus.Debugf("try parse(%d): %s[0:%d] = %s", count, line, scheduleEnds, line[0:scheduleEnds])

		expr, err := cronexpr.ParseStrict(line[:scheduleEnds])

		if err != nil {
			continue
		}

		return &CrontabLine{
			Expression: expr,
			Schedule:   line[:scheduleEnds],
			Command:    line[commandStarts:],
		}, nil
	}
	return nil, fmt.Errorf("bad crontab line: %s", line)
}

func ParseCrontab(reader io.Reader) (*Crontab, error) {
	scanner := bufio.NewScanner(reader)

	position := 0

	jobs := make([]*Job, 0)

	// TODO: CRON_TZ?
	environ := make(map[string]string)
	shell := "/bin/sh"

	for scanner.Scan() {
		line := strings.TrimLeft(scanner.Text(), " \t")

		if line == "" {
			continue
		}

		if line[0] == '#' {
			continue
		}

		r := envLineMatcher.FindAllStringSubmatch(line, -1)
		if len(r) == 1 && len(r[0]) == 3 {
			envKey := r[0][1]
			envVal := r[0][2]

			// Remove quotes (this emulates what Vixie cron does)
			if envVal[0] == '"' || envVal[0] == '\'' {
				if len(envVal) > 1 && envVal[0] == envVal[len(envVal)-1] {
					envVal = envVal[1 : len(envVal)-1]
				}
			}

			if envKey == "SHELL" {
				logrus.Infof("processes will be spawned using shell: %s", envVal)
				shell = envVal
			}

			if envKey == "USER" {
				logrus.Warnf("processes will NOT be spawned as USER=%s", envVal)
			}

			environ[envKey] = envVal

			continue
		}

		// replace start  with so the line will parse without errors
		jobLine, err := parseJobLine(strings.Replace(line, StartExpressionValue, "0 0 0 1 1 * *", 1))
		if err != nil {
			return nil, err
		}

		// set custom expression for startup job
		if strings.HasPrefix(line, StartExpressionValue) {
			jobLine.Expression = StartExpr
			jobLine.Schedule = StartExpressionValue
		}

		jobs = append(jobs, &Job{CrontabLine: *jobLine, Position: position})
		position++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &Crontab{
		Jobs: jobs,
		Context: &Context{
			Shell:   shell,
			Environ: environ,
		},
	}, nil
}
