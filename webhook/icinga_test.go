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
	"fmt"
	"math"
	"testing"

	"github.com/vshn/signalilo/config"
	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/prometheus/alertmanager/template"
	"github.com/stretchr/testify/assert"
)

type alertTestCase struct {
	alert       template.Alert
	nameCompute bool
	heartbeat   bool
	exitStatus  int
	interval    float64
	message     string
}

var (
	regularAlerts = map[string]alertTestCase{
		"basic service": {
			template.Alert{
				Labels: map[string]string{
					"alertname": "service_up",
					"service":   "testing",
				},
				Annotations: map[string]string{
					"message": "the message 0",
				},
			}, true, false, 0, 0, "the message 0",
		},
	}
	heartbeatAlerts = map[string]alertTestCase{
		"heartbeat critical": {
			template.Alert{
				Status: "firing",
				Labels: map[string]string{
					"alertname": "heartbeat",
					"service":   "testing",
					"heartbeat": "60s",
					"severity":  "critical",
				},
				Annotations: map[string]string{
					"message": "the message 1",
				},
			}, true, true, 2, 66, "the message 1",
		},
		"heartbeat warning": {
			template.Alert{
				Status: "firing",
				Labels: map[string]string{
					"alertname": "heartbeat2",
					"service":   "testing",
					"heartbeat": "5m",
					"severity":  "warning",
				},
				Annotations: map[string]string{
					"message": "the message 2",
				},
			}, true, true, 1, 330, "the message 2",
		},
	}
	serviceNames = map[string]struct {
		name string
		ok   bool
	}{
		// empty string should fail validation
		"empty string": {"", false},
		// simple service names should pass validation
		"simple name 1": {"a", true},
		"simple name 2": {"service", true},
		"simple name 3": {"x", true},
		// no restrictions on leading special characters, should pass
		// validation
		"leading special char": {"-b", true},
		// @ is not allowed, should fail validation
		"no @": {"hello@example.com", false},
		// the next name contains all the character classes that are
		// allowed and should pass
		"all allowed character classes": {"0+9:aA.bcZ,test_1", true},
		// exactly 128 characters, should pass
		"128 chars": {"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},
		// 129 characters is longer than allowed, should fail
		"129 chars": {"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab", false},
	}
)

func TestValidateServiceName(t *testing.T) {
	for name, tcase := range serviceNames {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, validateServiceName(tcase.name), tcase.ok, "service name validation works correctly")
		})
	}
}

func TestComputeServiceName(t *testing.T) {

	c := config.NewMockConfiguration(1)

	for name, tcase := range regularAlerts {
		t.Run(name, func(t *testing.T) {
			alert := tcase.alert
			_, err := computeServiceName(template.Data{}, alert, c)
			assert.Equal(t, err == nil, tcase.nameCompute, "service name computation successful")
		})
	}
	for name, tcase := range heartbeatAlerts {
		t.Run(name, func(t *testing.T) {
			alert := tcase.alert
			_, err := computeServiceName(template.Data{}, alert, c)
			assert.Equal(t, err == nil, tcase.nameCompute, "service name computation successful")
		})
	}
}

func TestUpdateOrCreateRegularService(t *testing.T) {
	c := config.NewMockConfiguration(1)
	i := icinga2.NewMockClient()

	for name, tcase := range regularAlerts {
		t.Run(name, func(t *testing.T) {
			alert := tcase.alert
			svcName, err := computeServiceName(template.Data{}, alert, c)
			displayName, err := computeDisplayName(template.Data{}, alert)
			svc, err := updateOrCreateService(i, "test.vshn.net", svcName, displayName, alert, c)
			assert.Equal(t, err == nil, true, fmt.Sprintf("Alert: %+v -> %v; err = %v", alert, svc, err))
			assert.Equal(t, svc.MaxCheckAttempts == 1, true, "soft states disabled for check %v", displayName)
			assert.Equal(t, svc.EnableActiveChecks, false, "active checks disabled")
			assert.Equal(t, svc.CheckInterval == 43200, true, "default check interval set")
		})
	}
}

func TestUpdateOrCreateHeartbeatService(t *testing.T) {
	c := config.NewMockConfiguration(1)
	i := icinga2.NewMockClient()

	for name, tcase := range heartbeatAlerts {
		t.Run(name, func(t *testing.T) {
			alert := tcase.alert
			svcName, err := computeServiceName(template.Data{}, alert, c)
			displayName, err := computeDisplayName(template.Data{}, alert)
			svc, err := updateOrCreateService(i, "test.vshn.net", svcName, displayName, alert, c)
			assert.Equal(t, err == nil, true, "service creation successful")
			assert.Equal(t, svc.MaxCheckAttempts == 1, true, "soft states disabled")
			assert.Equal(t, svc.EnableActiveChecks, true, "active checks enabled")
			// do math.Abs here to account for fp inaccuracies
			// when testing for equality
			assert.Equal(t, math.Abs(svc.CheckInterval-tcase.interval) < 0.0001, true, "check interval correct")
			dummyText, ok := svc.Vars["dummy_text"]
			dummyState, ok := svc.Vars["dummy_state"]
			assert.Equal(t, ok, true, "Dummy text is set")
			assert.Equal(t, ok, true, "Dummy state is set")
			assert.Equal(t, dummyState == tcase.exitStatus, true, "dummy_state of heartbeat is correct")
			assert.Equal(t, dummyText == tcase.message, true, "dummy_text of heartbeat is correct")
		})
	}
}
