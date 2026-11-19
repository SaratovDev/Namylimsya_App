package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProdWarnings_SMTPDisabled(t *testing.T) {
	cfg := &Config{}
	cfg.SMTP.Enabled = false

	warns := prodWarnings(cfg)
	require.NotEmpty(t, warns)
	require.Contains(t, warns[0], "SMTP_ENABLED=false")
}

func TestProdWarnings_SMTPEnabled(t *testing.T) {
	cfg := &Config{}
	cfg.SMTP.Enabled = true

	warns := prodWarnings(cfg)
	require.Empty(t, warns)
}
