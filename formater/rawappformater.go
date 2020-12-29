package formater

import (
	"github.com/sirupsen/logrus"
)

// RawAppFormatter formats logs into parsable json
type RawAppFormatter struct {
}

// Format renders a single log entry
func (f *RawAppFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(entry.Message + "\r\n"), nil
}
