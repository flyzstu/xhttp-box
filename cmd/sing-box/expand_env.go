package main

import (
	"os"
	"regexp"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
)

var envVariablePattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func expandEnvVariables(content []byte, path string) ([]byte, error) {
	var missing []string
	expanded := envVariablePattern.ReplaceAllFunc(content, func(match []byte) []byte {
		name := string(match[2 : len(match)-1])
		value, found := os.LookupEnv(name)
		if !found {
			missing = append(missing, name)
			return match
		}
		return []byte(value)
	})
	if len(missing) > 0 {
		seen := make(map[string]bool)
		var uniqueMissing []string
		for _, name := range missing {
			if !seen[name] {
				seen[name] = true
				uniqueMissing = append(uniqueMissing, name)
			}
		}
		return nil, E.New("undefined environment variables in config at ", path, ": ", strings.Join(uniqueMissing, ", "))
	}
	return expanded, nil
}
