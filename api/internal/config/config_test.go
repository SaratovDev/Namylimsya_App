package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateProdSMTP_Disabled(t *testing.T) {
	cfg := &Config{}
	cfg.SMTP.Enabled = false
	require.Error(t, validateProdSMTP(cfg))
}

func TestValidateProdSMTP_Enabled(t *testing.T) {
	cfg := &Config{}
	cfg.SMTP.Enabled = true
	require.NoError(t, validateProdSMTP(cfg))
}
