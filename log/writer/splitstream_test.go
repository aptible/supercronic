package writer

import (
	"testing"
	"os"
	"bytes"
	"io"
	"github.com/sirupsen/logrus"
	"strings"
)

func TestTextSplitStreamWriter_Write(t *testing.T) {

	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetOutput(&TextSplitStreamWriter{})

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	er, ew, _ := os.Pipe()
	sr, sw, _ := os.Pipe()
	os.Stdout = sw
	os.Stderr = ew

	outChan := make(chan string, 2)
	go func() {
		var stdoutBuff bytes.Buffer
		io.Copy(&stdoutBuff, sr)
		outChan <- stdoutBuff.String()
		outChan <- stdoutBuff.String()
	}()

	errChan := make(chan string, 2)
	go func() {
		var stderrOut bytes.Buffer
		io.Copy(&stderrOut, er)
		errChan <- stderrOut.String()
		errChan <- stderrOut.String()
	}()

	logrus.Debug("out1")
	logrus.Info("out2")
	logrus.Warn("err1")
	logrus.Error("err2")

	sw.Close()
	ew.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	stdoutMsg := <-outChan
	stderrMsg := <-errChan
	if !strings.Contains(stdoutMsg, "msg=out1") {
		t.Fatalf("failed to split info level log into stdout")
	}
	if !strings.Contains(stderrMsg, "msg=err1") {
		t.Fatalf("failed to split error level log into stderr")
	}

	stdoutMsg = <-outChan
	stderrMsg = <-errChan
	if !strings.Contains(stdoutMsg, "msg=out2") {
		t.Fatalf("failed to split info level log into stdout")
	}
	if !strings.Contains(stderrMsg, "msg=err2") {
		t.Fatalf("failed to split error level log into stderr")
	}
}

func TestJsonSplitStreamWriter_Write(t *testing.T) {

	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(&JsonSplitStreamWriter{})

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	er, ew, _ := os.Pipe()
	sr, sw, _ := os.Pipe()
	os.Stdout = sw
	os.Stderr = ew

	outChan := make(chan string, 2)
	go func() {
		var stdoutBuff bytes.Buffer
		io.Copy(&stdoutBuff, sr)
		outChan <- stdoutBuff.String()
		outChan <- stdoutBuff.String()
	}()

	errChan := make(chan string, 2)
	go func() {
		var stderrOut bytes.Buffer
		io.Copy(&stderrOut, er)
		errChan <- stderrOut.String()
		errChan <- stderrOut.String()
	}()

	logrus.Debug("out1")
	logrus.Info("out2")
	logrus.Warn("err1")
	logrus.Error("err2")

	sw.Close()
	ew.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	stdoutMsg := <-outChan
	stderrMsg := <-errChan
	if !strings.Contains(stdoutMsg, "\"msg\":\"out1\"") {
		t.Fatalf("failed to split info level log into stdout")
	}
	if !strings.Contains(stderrMsg, "\"msg\":\"err1\"") {
		t.Fatalf("failed to split error level log into stderr")
	}

	stdoutMsg = <-outChan
	stderrMsg = <-errChan
	if !strings.Contains(stdoutMsg, "\"msg\":\"out2\"") {
		t.Fatalf("failed to split info level log into stdout")
	}
	if !strings.Contains(stderrMsg, "\"msg\":\"err2\"") {
		t.Fatalf("failed to split error level log into stderr")
	}
}
