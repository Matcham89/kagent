package deepagents_test

import (
	"testing"

	"github.com/kagent-dev/kagent/go/core/pkg/sandboxbackend/openshell/deepagents"
	"github.com/stretchr/testify/require"
)

func TestDefaultSSHLaunchCommand(t *testing.T) {
	require.Equal(t, "cd /sandbox && HOME=/sandbox exec dcode", deepagents.DefaultSSHLaunchCommand())
}
