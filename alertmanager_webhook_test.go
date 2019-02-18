package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/corvus-ch/logr/buffered"
)

func isJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func TestAsJSON(t *testing.T) {
	rr := httptest.NewRecorder()

	asJSON(rr, http.StatusOK, "ok")

	// verify response properties
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("asJSON set wrong status code: got %v, want %v",
			status, http.StatusOK)
	}

	response := rr.Body.String()
	if !isJSON(response) {
		t.Errorf("asJSON returned something that isn't JSON: %v",
			response)
	}

	responseJSON := responseJSON{}
	if err := json.NewDecoder(rr.Body).Decode(&responseJSON); err != nil {
		t.Errorf("asJSON returned something that can't be decoded as responseJSON: %v",
			response)
	}
	if responseJSON.Status != http.StatusOK {
		t.Errorf("asJSON mangled response status: expected %v, got %v",
			http.StatusOK, responseJSON.Status)
	}
	if responseJSON.Message != "ok" {
		t.Errorf("asJSON mangled response status: expected %v, got %v",
			"ok", responseJSON.Message)
	}
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
