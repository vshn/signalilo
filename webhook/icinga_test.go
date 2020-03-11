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
	"strconv"
	"testing"

	"git.vshn.net/appuio/signalilo/config"
	"github.com/Nexinto/go-icinga2-client/icinga2"
	"github.com/prometheus/alertmanager/template"
	"github.com/stretchr/testify/assert"
)

func TestValidateServiceName(t *testing.T) {
	serviceNames := map[string]bool{
		// empty string should fail validation
		"": false,
		// simple service names should pass validation
		"a":       true,
		"service": true,
		"x":       true,
		// no restrictions on leading special characters, should pass
		// validation
		"-b": true,
		// @ is not allowed, should fail validation
		"hello@example.com": false,
		// the next name contains all the character classes that are
		// allowed and should pass
		"0+9:aA.bcZ,test_1": true,
		// exactly 128 characters, should pass
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": true,
		// 129 characters is longer than allowed, should fail
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab": false,
	}

	for name, expected := range serviceNames {
		assert.Equal(t, validateServiceName(name), expected, "service name regex works")
	}
}

func TestComputeServiceName(t *testing.T) {
	alerts := []template.Alert{
		template.Alert{
			Labels: map[string]string{
				"alertname": "service_up",
				"service":   "testing",
			},
			Annotations: map[string]string{
				"expected": "true",
			},
		},
	}

	c := config.NewMockConfiguration(1)

	for _, alert := range alerts {
		svcName, err := computeServiceName(template.Data{}, alert, c)
		expected, _ := strconv.ParseBool(alert.Annotations["expected"])
		assert.Equal(t, err == nil, expected, fmt.Sprintf("Alert: %+v -> %v; err = %v", alert, svcName, err))
	}
}

func TestUpdateOrCreateService(t *testing.T) {
	alerts := []template.Alert{
		template.Alert{
			Labels: map[string]string{
				"alertname": "service_up",
				"service":   "testing",
			},
			Annotations: map[string]string{
				"heartbeat": "false",
				"message":   "the message 0",
			},
		},
		template.Alert{
			Status: "firing",
			Labels: map[string]string{
				"alertname": "heartbeat",
				"service":   "testing",
				"heartbeat": "60s",
				"severity":  "critical",
			},
			Annotations: map[string]string{
				"heartbeat":  "true",
				"exitStatus": "2",
				"interval":   "66",
				"message":    "the message 1",
			},
		},
		template.Alert{
			Status: "firing",
			Labels: map[string]string{
				"alertname": "heartbeat2",
				"service":   "testing",
				"heartbeat": "5m",
				"severity":  "warning",
			},
			Annotations: map[string]string{
				"heartbeat":  "true",
				"exitStatus": "1",
				"interval":   "330",
				"message":    "the message 2",
			},
		},
	}
	c := config.NewMockConfiguration(1)
	i := icinga2.NewMockClient()

	for idx, alert := range alerts {
		svcName, err := computeServiceName(template.Data{}, alert, c)
		displayName, err := computeDisplayName(template.Data{}, alert)
		svc, err := updateOrCreateService(i, "test.vshn.net",
			svcName, displayName, alert, c)
		assert.Equal(t, err == nil, true, fmt.Sprintf("Alert: %+v -> %v; err = %v", alert, svc, err))
		assert.Equal(t, svc.MaxCheckAttempts == 1, true, "soft states disabled for check %v", displayName)
		isHeartbeat, _ := strconv.ParseBool(alert.Annotations["heartbeat"])
		var state, check string
		if isHeartbeat {
			state = "enabled"
			check = "heartbeat"
		} else {
			state = "disabled"
			check = "regular"
		}
		assert.Equal(t, svc.EnableActiveChecks, isHeartbeat,
			fmt.Sprintf("Active checking is %v on %v check %v", state, check, displayName))
		if isHeartbeat {
			checkInterval, _ := strconv.ParseFloat(alert.Annotations["interval"], 64)
			assert.Equal(t, math.Abs(svc.CheckInterval-checkInterval) < 0.001,
				isHeartbeat, "Check interval is correct on heartbeat check %v", displayName)
			dummyText, ok := svc.Vars["dummy_text"]
			assert.Equal(t, ok, true, "Dummy text is set on heartbeat check %v", displayName)
			dummyState, ok := svc.Vars["dummy_state"]
			assert.Equal(t, ok, true, "Dummy state is set on heartbeat check %v", displayName)
			exitStatus, _ := strconv.Atoi(alert.Annotations["exitStatus"])
			assert.Equal(t, dummyState == exitStatus, true,
				"dummy_state of heartbeat is configured correctly for %v", displayName)
			msg := fmt.Sprintf("the message %d", idx)
			assert.Equal(t, dummyText == msg, true,
				"dummy_text of heartbeat is configured correctly for %v", displayName)
		} else {
			assert.Equal(t, svc.CheckInterval == 43200, true)
		}
	}
}
