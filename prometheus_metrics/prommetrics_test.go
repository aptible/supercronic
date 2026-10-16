package prometheus_metrics

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestGetAddr(t *testing.T) {
	addr, err := getAddr("127.0.0.1:123")
	if assert.Nil(t, err) {
		assert.Equal(t, "127.0.0.1:123", addr)
	}

	addr, err = getAddr("127.0.0.1")
	if assert.Nil(t, err) {
		assert.Equal(t, "127.0.0.1:9746", addr)
	}

	addr, err = getAddr("[127.0.0.1]")
	if assert.Nil(t, err) {
		assert.Equal(t, "[127.0.0.1]:9746", addr)
	}

	addr, err = getAddr("[::]:123")
	if assert.Nil(t, err) {
		assert.Equal(t, "[::]:123", addr)
	}

	addr, err = getAddr("::")
	if assert.Nil(t, err) {
		assert.Equal(t, "[::]:9746", addr)
	}

	addr, err = getAddr("0.0.0.0")
	if assert.Nil(t, err) {
		assert.Equal(t, "0.0.0.0:9746", addr)
	}

	_, err = getAddr("")
	assert.NotNil(t, err)

	_, err = getAddr("[::]")
	assert.NotNil(t, err)
}

func TestInitHTTPServerShutdownIsNotFatal(t *testing.T) {
	logger := logrus.StandardLogger()
	originalExit := logger.ExitFunc
	exited := make(chan int, 1)
	logger.ExitFunc = func(code int) { exited <- code }
	defer func() { logger.ExitFunc = originalExit }()

	shutdown, err := InitHTTPServer("127.0.0.1:0", context.Background())
	if !assert.Nil(t, err) {
		return
	}

	assert.Nil(t, shutdown())

	select {
	case code := <-exited:
		t.Fatalf("shutting the server down logged a fatal error and exited with %d", code)
	case <-time.After(500 * time.Millisecond):
	}
}
