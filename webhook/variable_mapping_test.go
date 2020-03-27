package webhook

import (
	"sort"
	"strings"
	"testing"

	"github.com/vshn/go-icinga2-client/icinga2"
	"github.com/corvus-ch/logr/buffered"
	"github.com/stretchr/testify/assert"
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
	expected_errs := []string{
		"INFO Failed to map Icinga variable 'icinga_unknown_d': unknown type",
		"INFO Failed to map Icinga variable 'icinga_number_e': strconv.Atoi: parsing \"e\": invalid syntax",
	}
	sort.Strings(expected_errs)
	actual_errs := strings.Split(strings.TrimSpace(l.Buf().String()), "\n")
	sort.Strings(actual_errs)

	assert.Equal(t, strings.Join(expected_errs, "\n"), strings.Join(actual_errs, "\n"))
}
