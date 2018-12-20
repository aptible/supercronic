package hook

import (
	"bytes"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestSplitStdoutStreamHook_Fire(t *testing.T) {

	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(ioutil.Discard)
	logrus.AddHook(&SplitStdoutStreamHook{})

	oldStdout := os.Stdout
	sr, sw, _ := os.Pipe()
	os.Stdout = sw

	outChan := make(chan string, 2)
	go func() {
		var stdoutBuff bytes.Buffer
		io.Copy(&stdoutBuff, sr)
		outChan <- stdoutBuff.String()
		outChan <- stdoutBuff.String()
	}()

	logrus.Debug("out1")
	logrus.Info("out2")

	sw.Close()
	os.Stdout = oldStdout

	stdoutMsg := <-outChan
	if !strings.Contains(stdoutMsg, "msg=out1") {
		t.Fatalf("failed to split info level log into stdout")
	}

	stdoutMsg = <-outChan
	if !strings.Contains(stdoutMsg, "msg=out2") {
		t.Fatalf("failed to split info level log into stdout")
	}
}

func TestSplitStderrStreamHook_Fire(t *testing.T) {
	logrus.SetOutput(ioutil.Discard)
	logrus.AddHook(&SplitStderrStreamHook{})

	oldStderr := os.Stderr
	er, ew, _ := os.Pipe()
	os.Stderr = ew

	errChan := make(chan string, 2)
	go func() {
		var stderrOut bytes.Buffer
		io.Copy(&stderrOut, er)
		errChan <- stderrOut.String()
		errChan <- stderrOut.String()
	}()

	logrus.Warn("err1")
	logrus.Error("err2")

	ew.Close()
	os.Stderr = oldStderr

	stderrMsg := <-errChan
	if !strings.Contains(stderrMsg, "msg=err1") {
		t.Fatalf("failed to split error level log into stderr")
	}

	stderrMsg = <-errChan
	if !strings.Contains(stderrMsg, "msg=err2") {
		t.Fatalf("failed to split error level log into stderr")
	}
}
