package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/corvus-ch/logr/buffered"
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

func checkLogMessage(t *testing.T, buf *bytes.Buffer, expected_msg string) {
	actual_msg := strings.TrimSpace(buf.String())
	if actual_msg != expected_msg {
		t.Errorf("Log message: expected '%v', got '%v'", expected_msg, actual_msg)
	}
	// clear logger buffer
	buf.Reset()
}

func TestCheckOrCreateHost(t *testing.T) {
	icinga := icinga2.NewMockClient()
	l := buffered.New(0)
	hostname := "testhost1.local"

	host, err := checkOrCreateHost(icinga, hostname, l)
	if err != nil {
		t.Errorf("Error creating host: %v", err)
	}
	checkLogMessage(t, l.Buf(), fmt.Sprintf("INFO creating host: %+v", hostname))
	found := false
	for _, h := range icinga.Hosts {
		if h.Name == host.Name {
			found = true
		}
	}
	if !found {
		t.Errorf("Unable to find host in icinga mock client")
	}

	// do checkOrCreate again -> should return same host struct and log
	// "found host"
	host2, err := checkOrCreateHost(icinga, "testhost1.local", l)
	if err != nil {
		t.Errorf("Error checking host: %v", err)
	}
	checkLogMessage(t, l.Buf(), fmt.Sprintf("INFO found host: %+v", hostname))
	if host2.Name != host.Name {
		t.Errorf("Created and found host name mismatch: expected %v, got %v\n",
			host.Name, host2.Name)
	}
}
