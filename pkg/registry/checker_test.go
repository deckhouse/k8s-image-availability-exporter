package registry

import (
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseImageName(t *testing.T) {
	const (
		goodImageName                = "docker.io/test:test"
		goodImageNameWithoutRegistry = "test:test"
		badImageName                 = "te*^#@@st"

		defaultRegistryName = "test-registry.io"
	)

	_, err := parseImageName(goodImageName, "")
	require.NoError(t, err)

	_, err = parseImageName(badImageName, "")
	require.Error(t, err)

	ref, err := parseImageName(goodImageNameWithoutRegistry, defaultRegistryName)
	require.NoError(t, err)
	require.Equal(t, ref.Name(), path.Join(defaultRegistryName, goodImageNameWithoutRegistry))
}
