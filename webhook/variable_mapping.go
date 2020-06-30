package webhook

import (
	"errors"
	"regexp"
	"strconv"

	"github.com/bketelsen/logr"
	"github.com/vshn/go-icinga2-client/icinga2"
)

var (
	ErrorNotAMappingKey     = errors.New("key does meet the mappable pattern")
	ErrorUnknownMappingType = errors.New("unknown type")
	MappingKeyPattern       = regexp.MustCompile("^icinga_([a-z]+)_(.*)$")
)

func mapIcingaVariables(vars icinga2.Vars, kv map[string]string, prefix string, log logr.Logger) icinga2.Vars {
	for k, v := range kv {
		vars[prefix+k] = v

		kk, vv, err := mapIcingaVariable(k, v)
		if err == ErrorNotAMappingKey {
			continue
		} else if err != nil {
			log.Infof("Failed to map Icinga variable '%s': %s", k, err)
			continue
		}

		vars[kk] = vv
	}

	return vars
}

func mapIcingaVariable(key, value string) (string, interface{}, error) {
	matches := MappingKeyPattern.FindStringSubmatch(key)
	if len(matches) < 3 {
		return key, value, ErrorNotAMappingKey
	}
	t := matches[1]
	k := matches[2]

	switch t {
	case "number":
		v, err := strconv.Atoi(value)
		if err != nil {
			return "", nil, err
		}
		return k, v, nil

	case "string":
		return k, value, nil
	}

	return "", nil, ErrorUnknownMappingType
}
