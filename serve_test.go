package main

import (
	"git.vshn.net/appuio/signalilo/config"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestHealthz(t *testing.T) {
	// grab mock config
	s := &ServeCommand{}
	s.logger = config.MockLogger(1)

	assert := assert.New(t)

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) { healthz(w, r, s) })

	assert.HTTPSuccess(handler, "GET", "http://example.com/healthz", nil)

	assert.HTTPBodyContains(handler, "GET", "http://example.com/healthz", nil, "ok")
}
