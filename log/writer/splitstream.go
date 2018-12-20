package writer

import (
	"bytes"
	"os"
	"github.com/sirupsen/logrus"
	"encoding/json"
)

type TextSplitStreamWriter struct {}
func (ss *TextSplitStreamWriter) Write(entry []byte) (n int, err error) {
	if bytes.Contains(entry, []byte("level=debug")) || bytes.Contains(entry, []byte("level=info")) {
		return os.Stdout.Write(entry)
	}
	return os.Stderr.Write(entry)
}

type JsonSplitStreamWriter struct {}
func (ss *JsonSplitStreamWriter) Write(entry []byte) (n int, err error) {
	var log map[string]interface{}
	json.Unmarshal(entry, &log)
	if level, ok := log[logrus.FieldKeyLevel]; ok {
		if level == "debug" || level == "info" {
			return os.Stdout.Write(entry)
		}
	}
	return os.Stderr.Write(entry)
}


