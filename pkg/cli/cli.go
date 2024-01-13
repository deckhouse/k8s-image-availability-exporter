package cli

import (
	"fmt"
	"slices"
	"strings"
)

type ForceCheckDisabledControllerKindsParser struct {
	allowedControllerKinds []string
	ParsedKinds            []string
}

func (parser *ForceCheckDisabledControllerKindsParser) Parse(flagValue string) error {
	if flagValue == "*" {
		parser.ParsedKinds = parser.allowedControllerKinds
		return nil
	}

	parser.ParsedKinds = []string{}

flagLoop:
	for _, kind := range strings.Split(flagValue, ",") {
		kind = strings.ToLower(kind)

		for _, allowedControllerKind := range parser.allowedControllerKinds {
			if kind == allowedControllerKind {
				if !slices.Contains(parser.ParsedKinds, kind) {
					parser.ParsedKinds = append(parser.ParsedKinds, kind)
				}
				continue flagLoop
			}
		}

		return fmt.Errorf(`must be one of %s or * for all kinds`, strings.Join(parser.allowedControllerKinds, ", "))
	}

	return nil
}

func NewForceCheckDisabledControllerKindsParser() *ForceCheckDisabledControllerKindsParser {
	parser := &ForceCheckDisabledControllerKindsParser{}
	parser.allowedControllerKinds = []string{"deployment", "statefulset", "daemonset", "cronjob"}
	return parser
}
