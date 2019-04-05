package webhook

import (
	"encoding/json"
	"net/http"
	"testing"

	tassert "github.com/stretchr/testify/assert"
)

func isJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func mockEchoHandler(w http.ResponseWriter, r *http.Request) {
	asJSON(w, http.StatusOK, "ok")
}

func TestAsJSON(t *testing.T) {
	assert := tassert.New(t)

	handler := http.HandlerFunc(mockEchoHandler)

	// verify response properties
	assert.HTTPSuccess(handler, "GET", "http://example.com/webhook", nil)
	response := tassert.HTTPBody(handler, "GET", "http://example.com/webhook", nil)
	assert.JSONEq(response, `{ "Status": 200, "Message": "ok" }`)
}
