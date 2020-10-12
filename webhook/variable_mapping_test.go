package webhook

import (
	"sort"
	"strings"
	"testing"

	"github.com/corvus-ch/logr/buffered"
	"github.com/stretchr/testify/assert"
	"github.com/vshn/go-icinga2-client/icinga2"
)

var mapIcingaVariableTest = map[string]struct {
	iK  string
	iV  string
	oK  string
	oV  interface{}
	err error
}{
	"not mapped":    {"foo", "bar", "foo", "bar", ErrorNotAMappingKey},
	"mapped number": {"icinga_number_foo", "42", "foo", 42, nil},
	"mapped string": {"icinga_string_foo", "bar", "foo", "bar", nil},
	"unknown":       {"icinga_unknown_foo", "bar", "", nil, ErrorUnknownMappingType},
}

func TestMapIcingaVariable(t *testing.T) {
	for name, test := range mapIcingaVariableTest {
		t.Run(name, func(t *testing.T) {
			k, v, err := mapIcingaVariable(test.iK, test.iV)
			assert.Equal(t, test.err, err)
			assert.Equal(t, test.oK, k)
			assert.Equal(t, test.oV, v)
		})
	}
}

func TestMapIcingaVariables(t *testing.T) {
	vars := make(icinga2.Vars)
	kv := map[string]string{
		"a":                "a",
		"icinga_number_b":  "42",
		"icinga_string_c":  "c",
		"icinga_unknown_d": "d",
		"icinga_number_e":  "e",
	}
	l := buffered.New(0)
	vars = mapIcingaVariables(vars, kv, "pre_", l)
	assert.Equal(t, icinga2.Vars{
		"pre_a":                "a",
		"pre_icinga_number_b":  "42",
		"pre_icinga_string_c":  "c",
		"pre_icinga_unknown_d": "d",
		"pre_icinga_number_e":  "e",
		"b":                    42,
		"c":                    "c",
	}, vars)
	expectedErrs := []string{
		"INFO Failed to map Icinga variable 'icinga_unknown_d': unknown type",
		"INFO Failed to map Icinga variable 'icinga_number_e': strconv.Atoi: parsing \"e\": invalid syntax",
	}
	sort.Strings(expectedErrs)
	actualErrs := strings.Split(strings.TrimSpace(l.Buf().String()), "\n")
	sort.Strings(actualErrs)

	assert.Equal(t, strings.Join(expectedErrs, "\n"), strings.Join(actualErrs, "\n"))
}

func TestAddStaticVariables(t *testing.T) {
	vars := make(icinga2.Vars)
	l := buffered.New(0)
	staticVars := map[string]string{
		"a": "a",
		"b": "b",
	}
	vars = addStaticIcingaVariables(vars, staticVars, l)
	assert.Equal(t, icinga2.Vars{
		"a": "a",
		"b": "b",
	}, vars)
}

func TestAddStaticVariablesNoOverwrite(t *testing.T) {
	vars := make(icinga2.Vars)
	vars["a"] = "z"
	l := buffered.New(0)
	staticVars := map[string]string{
		"a": "a",
		"b": "b",
	}
	vars = addStaticIcingaVariables(vars, staticVars, l)
	assert.Equal(t, icinga2.Vars{
		"a": "z",
		"b": "b",
	}, vars)
}
