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

	_, err := parseImageName(goodImageName, "", false)
	require.NoError(t, err)

	_, err = parseImageName(badImageName, "", false)
	require.Error(t, err)

	ref, err := parseImageName(goodImageNameWithoutRegistry, defaultRegistryName, false)
	require.NoError(t, err)
	require.Equal(t, path.Join(defaultRegistryName, goodImageNameWithoutRegistry), ref.Name())
}
