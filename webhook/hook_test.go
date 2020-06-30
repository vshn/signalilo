/*
 * Authors:
 * Simon Gerber <simon.gerber@vshn.ch>
 *
 * License:
 * Copyright (c) 2019, VSHN AG, <info@vshn.ch>
 * Licensed under "BSD 3-Clause". See LICENSE file.
 */

package webhook

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/signalilo/config"
)

func isJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func mockEchoHandler(w http.ResponseWriter, r *http.Request) {
	asJSON(w, http.StatusOK, "ok")
}

func TestAsJSON(t *testing.T) {
	handler := http.HandlerFunc(mockEchoHandler)

	// verify response properties
	assert.HTTPSuccess(t, handler, "GET", "http://example.com/webhook", nil)
	response := assert.HTTPBody(handler, "GET", "http://example.com/webhook", nil)
	assert.JSONEq(t, response, `{ "Status": 200, "Message": "ok" }`)
}

func TestBearerTokenHeader(t *testing.T) {
	conf := config.NewMockConfiguration(1)
	req, _ := http.NewRequest(http.MethodPost, "https://example.com/webhook", nil)
	req.Header.Add("Authorization", "Bearer "+conf.GetConfig().AlertManagerConfig.BearerToken)
	err := checkBearerToken(req, conf)
	assert.NoError(t, err)
}

func TestBearerTokenQueryParam(t *testing.T) {
	conf := config.NewMockConfiguration(1)
	req, _ := http.NewRequest(http.MethodPost, "https://example.com/webhook?token="+conf.GetConfig().AlertManagerConfig.BearerToken, nil)
	err := checkBearerToken(req, conf)
	assert.NoError(t, err)
}

func TestBearerTokenMissing(t *testing.T) {
	conf := config.NewMockConfiguration(1)
	req, _ := http.NewRequest(http.MethodPost, "https://example.com/webhook", nil)
	err := checkBearerToken(req, conf)
	assert.Error(t, err)
}
