package hook

import (
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type testWriter struct {
	c chan []byte
}

func (w testWriter) Write(p []byte) (int, error) {
	w.c <- p
	return len(p), nil
}

func TestSplitStdoutStreamHook_Fire(t *testing.T) {
	outWriter := testWriter{c: make(chan []byte, 2)}
	errWriter := testWriter{c: make(chan []byte, 2)}
	defaultWriter := testWriter{c: make(chan []byte, 2)}

	log := logrus.New()
	log.SetOutput(defaultWriter)
	log.SetLevel(logrus.DebugLevel)

	RegisterSplitLogger(log, outWriter, errWriter)

	log.Debug("out1")
	log.Info("out2")
	log.Warn("err1")
	log.Error("err2")

	select {
	case log := <-outWriter.c:
		assert.Contains(t, string(log), "out1")
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for out log")
	}

	select {
	case log := <-outWriter.c:
		assert.Contains(t, string(log), "out2")
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for out log")
	}

	select {
	case log := <-errWriter.c:
		assert.Contains(t, string(log), "err1")
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for err log")
	}

	select {
	case log := <-errWriter.c:
		assert.Contains(t, string(log), "err2")
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for err log")
	}

	select {
	case <-defaultWriter.c:
		t.Fatalf("got default log")
	case <-time.After(time.Second):
		// Noop
	}
}
