package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
