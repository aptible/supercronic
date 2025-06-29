package formatter

import (
	"github.com/sirupsen/logrus"
	"regexp"
	"strings"
	"time"
)

type CustomFieldFormatter struct {
	LogFormat string
}

func (f *CustomFieldFormatter) getFieldValue(entry *logrus.Entry, field string) (string, bool) {
	switch strings.ToLower(field) {
	case "level":
		return entry.Level.String(), true
	case "time":
		return entry.Time.Format(time.RFC3339Nano), true
	case "message":
		return entry.Message, true
	default:
		val, ok := entry.Data[field]

		if ok {
			return val.(string), true
		}

		return "", false
	}
}

func (f *CustomFieldFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	re := regexp.MustCompile(`%[\w.]+`)

	replaced := re.ReplaceAllStringFunc(f.LogFormat, func(match string) string {
		// Remove the $ prefix to get the key for valuesMap
		key := strings.TrimPrefix(match, "%")
		// If the key exists in the entry, replace with the value from the map
		if value, ok := f.getFieldValue(entry, key); ok {
			return value
		}

		return ""
	})

	return []byte(strings.TrimSpace(replaced) + "\n"), nil
}
