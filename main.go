package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/aptible/supercronic/cron"
	"github.com/aptible/supercronic/crontab"
	"github.com/evalphobia/logrus_sentry"
	"github.com/sirupsen/logrus"
)

var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] CRONTAB\n\nAvailable options:\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	debug := flag.Bool("debug", false, "enable debug logging")
	json := flag.Bool("json", false, "enable JSON logging")
	sentry := flag.String("sentryDsn", "", "enable Sentry error logging, using provided DSN")
	flag.Parse()

	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if *json {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	}

	var sentryHook *logrus_sentry.SentryHook
	if *sentry != "" {
		sentryLevels := []logrus.Level{
			logrus.PanicLevel,
			logrus.FatalLevel,
			logrus.ErrorLevel,
		}
		sh, err := logrus_sentry.NewSentryHook(*sentry, sentryLevels)
		if err != nil {
			logrus.Warningf("Could not init sentry logger: %s", err)
		} else {
			sh.Timeout = 5 * time.Second
			sentryHook = sh
		}
	}

	if flag.NArg() != 1 {
		Usage()
		os.Exit(2)
		return
	}

	crontabFileName := flag.Args()[0]
	logrus.Infof("read crontab: %s", crontabFileName)

	tab, err := readCrontabAtPath(crontabFileName)

	if err != nil {
		logrus.Fatal(err)
		return
	}

	var wg sync.WaitGroup
	exitCtx, notifyExit := context.WithCancel(context.Background())

	for _, job := range tab.Jobs {
		cronLogger := logrus.WithFields(logrus.Fields{
			"job.schedule": job.Schedule,
			"job.command":  job.Command,
			"job.position": job.Position,
		})

		if sentryHook != nil {
			cronLogger.Logger.AddHook(sentryHook)
		}

		cron.StartJob(&wg, tab.Context, job, exitCtx, cronLogger)
	}

	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)

	termSig := <-termChan

	logrus.Infof("received %s, shutting down", termSig)
	notifyExit()

	logrus.Info("waiting for jobs to finish")
	wg.Wait()

	logrus.Info("exiting")
}

func readCrontabAtPath(path string) (*crontab.Crontab, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return crontab.ParseCrontab(file)
}
