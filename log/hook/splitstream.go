package hook

import (
	"github.com/sirupsen/logrus"
	"os"
)

type SplitStdoutStreamHook struct{}

func (soh *SplitStdoutStreamHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.DebugLevel,
		logrus.InfoLevel,
	}
}
func (soh *SplitStdoutStreamHook) Fire(entry *logrus.Entry) error {
	serialized, err := entry.Logger.Formatter.Format(entry)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(serialized)
	return err
}

type SplitStderrStreamHook struct{}

func (seh *SplitStderrStreamHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}
func (seh *SplitStderrStreamHook) Fire(entry *logrus.Entry) error {
	serialized, err := entry.Logger.Formatter.Format(entry)
	if err != nil {
		return err
	}
	_, err = os.Stderr.Write(serialized)
	return err
}
