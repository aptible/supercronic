package main

import (
	"fmt"
	"bufio"
	"os"
	"github.com/aptible/supercronic/cron"
	"github.com/aptible/supercronic/crontab"
	"github.com/sirupsen/logrus"
)



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

	crontab, err := crontab.ParseCrontab(bufio.NewScanner(file))

	if (err != nil) {
		logrus.Fatal(err)
		return
	}

	// TODO: Signal handling.
	// TODO: Should actually have a sync group here, and send the exit
	// request in.
	requestExitChan := make(chan interface{})

	for _, job := range crontab.Jobs {
		go cron.StartJob(crontab.Context, job, requestExitChan)
	}

	<-requestExitChan
}
