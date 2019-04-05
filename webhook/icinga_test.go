package webhook

import (
	"fmt"
	"strconv"
	"testing"

	"git.vshn.net/appuio/signalilo/config"
	"github.com/prometheus/alertmanager/template"
	tassert "github.com/stretchr/testify/assert"
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

	assert := tassert.New(t)

	for name, expected := range serviceNames {
		assert.Equal(validateServiceName(name), expected, "service name regex works")
	}
}

func TestComputeServiceName(t *testing.T) {
	assert := tassert.New(t)

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

	c := config.NewMockConfiguration("./config.test.yaml", 1)

	for _, alert := range alerts {
		svcName, err := computeServiceName(template.Data{}, alert, c)
		expected, _ := strconv.ParseBool(alert.Annotations["expected"])
		assert.Equal(err == nil, expected, fmt.Sprintf("Alert: %+v -> %v; err = %v", alert, svcName, err))
	}
}
