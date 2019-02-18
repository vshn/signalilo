package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	// grab mock config
	c := MockConfig()

	// create request which we use to test the handler
	req, err := http.NewRequest("GET", "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}

	// create responserecorder to record the response of the handler
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { healthz(w, r, c) })
	// serve handler, and record response
	handler.ServeHTTP(rr, req)

	// verify response properties
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)
	}

	expected := "ok"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v, want %v",
			rr.Body.String(), expected)
	}
}
