package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/aptible/supercronic/cron"
	"github.com/aptible/supercronic/crontab"
	"github.com/aptible/supercronic/log/writer"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] CRONTAB\n\nAvailable options:\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	debug := flag.Bool("debug", false, "enable debug logging")
	json := flag.Bool("json", false, "enable JSON logging")
	splitStream := flag.Bool("split-stream", false, "split log stream into stdout and stderr")
	flag.Parse()

	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if *json {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	}

	if *splitStream {
		if *json {
			logrus.SetOutput(&writer.JsonSplitStreamWriter{})
		} else {
			logrus.SetOutput(&writer.TextSplitStreamWriter{})
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
