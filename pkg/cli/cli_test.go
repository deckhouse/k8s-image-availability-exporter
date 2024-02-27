package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ForceCheckDisabledControllerKindsParser(t *testing.T) {
	const (
		allKinds                = "*"
		goodKinds               = "deployment,statefulset"
		goodKindsWithDuplicates = "deployment,deployment,statefulset,cronjob,cronjob"
		goodKindsWithWildcard   = "deployment,statefulset,*"
		badKinds                = "deployment,job"
	)
	parser := NewForceCheckDisabledControllerKindsParser()
	expectedErr := fmt.Errorf(`must be one of %s or * for all kinds`, strings.Join(parser.allowedControllerKinds, ", "))

	err := parser.Parse(allKinds)
	require.NoError(t, err)
	require.Equal(t, parser.ParsedKinds, parser.allowedControllerKinds)

	err = parser.Parse(goodKinds)
	require.NoError(t, err)
	require.Equal(t, parser.ParsedKinds, []string{"deployment", "statefulset"})

	err = parser.Parse(goodKindsWithDuplicates)
	require.NoError(t, err)
	require.Equal(t, parser.ParsedKinds, []string{"deployment", "statefulset", "cronjob"})

	err = parser.Parse(goodKindsWithWildcard)
	require.Error(t, expectedErr, err)

	err = parser.Parse(badKinds)
	require.Error(t, expectedErr, err)
}
