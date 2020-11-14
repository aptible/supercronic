package crontab

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/krallin/cronexpr"
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

		toParse := line[:scheduleEnds]

		// TODO: Should receive a logger?
		logrus.Debugf("try parse (%d fields): '%s'", count, toParse)

		expr, err := cronexpr.ParseStrict(toParse)

		if err != nil {
			logrus.Debugf("failed to parse (%d fields): '%s': failed: %v", count, toParse, err)
			continue
		}

		return &CrontabLine{
			Expression: expr,
			Schedule:   line[:scheduleEnds],
			Command:    line[commandStarts:],
		}, nil
	}

	return nil, fmt.Errorf("bad crontab line: '%s' (use -debug for details)", line)
}

func ParseCrontab(reader io.Reader) (*Crontab, error) {
	scanner := bufio.NewScanner(reader)

	position := 0

	jobs := make([]*Job, 0)

	environ := make(map[string]string)
	shell := "/bin/sh"
	tz := time.Local

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

			if envKey == "CRON_TZ" {
				var err error
				tz, err = time.LoadLocation(envVal)
				if err != nil {
					return nil, err
				}
				logrus.Infof("processes will be spawned using TZ: %v", tz)
			}

			environ[envKey] = envVal

			continue
		}

		jobLine, err := parseJobLine(line)
		if err != nil {
			return nil, err
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
			Shell:    shell,
			Environ:  environ,
			Timezone: tz,
		},
	}, nil
}
