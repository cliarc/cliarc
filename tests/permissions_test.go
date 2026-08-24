package tests

import (
	"testing"

	"github.com/cliarc/cliarc/core/permissions"
	"github.com/stretchr/testify/assert"
)

func TestPermissionValidator(t *testing.T) {
	v := permissions.NewValidator()
	v.Register("ssh", []string{"server.read", "server.*"})

	assert.True(t, v.Check("ssh", "server.read"))
	assert.True(t, v.Check("ssh", "server.execute"))
	assert.False(t, v.Check("ssh", "billing.read"))
	assert.False(t, v.Check("other", "server.read"))
}

func TestValidateAction(t *testing.T) {
	v := permissions.NewValidator()
	v.Register("ssh", []string{"connection.test"})

	err := v.ValidateAction("ssh", "connection.test")
	assert.NoError(t, err)

	err = v.ValidateAction("ssh", "server.delete")
	assert.Error(t, err)
}
