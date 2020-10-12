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

	"github.com/prometheus/alertmanager/template"
	"github.com/stretchr/testify/assert"
	"github.com/vshn/go-icinga2-client/icinga2"
	"github.com/vshn/signalilo/config"
)

type alertTestCase struct {
	alert       template.Alert
	nameCompute bool
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
			}, true, 0, 0, "the message 0",
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
			}, true, 2, 66, "the message 1",
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
			}, true, 1, 330, "the message 2",
		},
	}
	resolvedHeartbeatAlerts = map[string]alertTestCase{
		"heartbeat resolved": {
			template.Alert{
				Status: "resolved",
				Labels: map[string]string{
					"alertname": "heartbeat3",
					"service":   "testing",
					"heartbeat": "5m",
					"severity":  "warning",
				},
				Annotations: map[string]string{
					"message": "the message 2",
				},
			}, true, 1, 330, "the message 2",
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
			assert.Equal(t, tcase.ok, validateServiceName(tcase.name), "service name validation works correctly")
		})
	}
}

func TestComputeServiceName(t *testing.T) {

	c := config.NewMockConfiguration(1)

	for name, tcase := range regularAlerts {
		t.Run(name, func(t *testing.T) {
			alert := tcase.alert
			_, err := computeServiceName(template.Data{}, alert, c)
			assert.Equal(t, tcase.nameCompute, err == nil, "service name computation successful")
		})
	}
	for name, tcase := range heartbeatAlerts {
		t.Run(name, func(t *testing.T) {
			alert := tcase.alert
			_, err := computeServiceName(template.Data{}, alert, c)
			assert.Equal(t, tcase.nameCompute, err == nil, "service name computation successful")
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
			assert.NoError(t, err)
			displayName, err := computeDisplayName(template.Data{}, alert)
			assert.NoError(t, err)
			svc, err := updateOrCreateService(i, "test.vshn.net", svcName, displayName, alert, c)
			assert.NoError(t, err, fmt.Sprintf("Alert: %+v -> %v; err = %v", alert, svc, err))
			assert.Equal(t, 1.0, svc.MaxCheckAttempts, "soft states disabled for check %v", displayName)
			assert.False(t, svc.EnableActiveChecks, "active checks disabled")
			assert.Equal(t, 43200.0, svc.CheckInterval, "default check interval set")
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
			assert.NoError(t, err)
			displayName, err := computeDisplayName(template.Data{}, alert)
			assert.NoError(t, err)
			svc, err := updateOrCreateService(i, "test.vshn.net", svcName, displayName, alert, c)
			assert.NoError(t, err, "service creation successful")
			assert.Equal(t, 1.0, svc.MaxCheckAttempts, "soft states disabled")
			assert.True(t, svc.EnableActiveChecks, "active checks enabled")
			// do math.Abs here to account for fp inaccuracies
			// when testing for equality
			assert.Equal(t, math.Abs(svc.CheckInterval-tcase.interval) < 0.0001, true, "check interval correct")
			dummyText, ok := svc.Vars["dummy_text"]
			assert.True(t, ok, "Dummy text is set")
			dummyState, ok := svc.Vars["dummy_state"]
			assert.True(t, ok, "Dummy state is set")
			assert.Equal(t, tcase.exitStatus, dummyState, "dummy_state of heartbeat is correct")
			assert.Equal(t, tcase.message, dummyText, "dummy_text of heartbeat is correct")
		})
	}
	for name, tcase := range resolvedHeartbeatAlerts {
		t.Run(name, func(t *testing.T) {
			alert := tcase.alert
			svcName, err := computeServiceName(template.Data{}, alert, c)
			assert.NoError(t, err)
			displayName, err := computeDisplayName(template.Data{}, alert)
			assert.NoError(t, err)
			svc, err := updateOrCreateService(i, "test.vshn.net", svcName, displayName, alert, c)
			assert.NoError(t, err, "service creation successful")
			assert.Equal(t, "", svc.Name, "no service object created for resolved heartbeat")
		})
	}
}
