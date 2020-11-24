package prometheus_metrics

import (
	"testing"

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
