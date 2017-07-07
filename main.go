package main

import (
	"fmt"
	"github.com/aptible/supercronic/cron"
	"github.com/aptible/supercronic/crontab"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	// TODO: debug flag instead
	// TODO: JSON logging?
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	if len(os.Args) != 2 {
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

	tab, err := crontab.ParseCrontab(file)

	if err != nil {
		logrus.Fatal(err)
		return
	}

	var (
		wg        sync.WaitGroup
		exitChans []chan interface{}
	)

	for _, job := range tab.Jobs {
		c := make(chan interface{}, 1)
		exitChans = append(exitChans, c)
		cron.StartJob(&wg, tab.Context, job, c)
	}

	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)

	termSig := <-termChan

	logrus.Infof("received %s, shutting down", termSig)
	for _, c := range exitChans {
		c <- true
	}

	logrus.Info("waiting for jobs to finish")
	wg.Wait()

	logrus.Info("exiting")
}
