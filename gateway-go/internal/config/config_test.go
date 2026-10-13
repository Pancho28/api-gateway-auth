package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigParsing(t *testing.T) {
	os.Setenv("PORT", "9000")
	os.Setenv("SECRET_KEY", "custom_secret")
	os.Setenv("DB_USER", "custom_user")
	os.Setenv("MICROSERVICES", `{"sales": {"url": "http://sales:8001", "api_key": "sales_key"}}`)
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("SECRET_KEY")
		os.Unsetenv("DB_USER")
		os.Unsetenv("MICROSERVICES")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "9000", cfg.Port)
	assert.Equal(t, "custom_secret", cfg.SecretKey)
	assert.Equal(t, "custom_user", cfg.DBUser)

	require.Contains(t, cfg.Microservices, "sales")
	assert.Equal(t, "http://sales:8001", cfg.Microservices["sales"].URL)
	assert.Equal(t, "sales_key", cfg.Microservices["sales"].APIKey)
}

func TestConfigInvalidMicroservicesJSON(t *testing.T) {
	os.Setenv("MICROSERVICES", `{invalid-json`)
	defer os.Unsetenv("MICROSERVICES")

	_, err := Load()
	assert.Error(t, err)
}
