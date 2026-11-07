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

func TestServiceConfigPartitionRules(t *testing.T) {
	// 1. Zero-trust / Deny by default: lista vacía deniega acceso
	emptySvc := ServiceConfig{
		URL:    "http://backend:8000",
		APIKey: "key",
	}
	assert.False(t, emptySvc.IsClientAllowed("app-comensales"))
	assert.False(t, emptySvc.IsClientAllowed(""))

	// 2. Lista blanca específica
	restrictedSvc := ServiceConfig{
		URL:            "http://backend:8000",
		APIKey:         "key",
		AllowedClients: []string{"app-staff", "app-pos"},
		RequiredRoles:  []string{"ADMIN"},
	}
	assert.True(t, restrictedSvc.IsClientAllowed("app-staff"))
	assert.True(t, restrictedSvc.IsClientAllowed("app-pos"))
	assert.False(t, restrictedSvc.IsClientAllowed("app-comensales"))
	assert.False(t, restrictedSvc.IsClientAllowed(""))

	// Rol permitido vs denegado
	assert.True(t, restrictedSvc.IsRoleAllowed("ADMIN"))
	assert.True(t, restrictedSvc.IsRoleAllowed("admin")) // Case-insensitive
	assert.False(t, restrictedSvc.IsRoleAllowed("USER"))

	// 3. Comodín ("*"): permite cualquier cliente
	wildcardSvc := ServiceConfig{
		URL:            "http://backend:8000",
		APIKey:         "key",
		AllowedClients: []string{"*"},
	}
	assert.True(t, wildcardSvc.IsClientAllowed("app-comensales"))
	assert.True(t, wildcardSvc.IsClientAllowed("cualquier-cliente"))
}

func TestConfigPassthroughHeadersParsing(t *testing.T) {
	os.Setenv("PORT", "8000")
	os.Setenv("SECRET_KEY", "test-secret")
	os.Setenv("MICROSERVICES", `{
		"reservas": {
			"url": "http://localhost:4000",
			"api_key": "key-123",
			"allowed_clients": ["reservas-web"],
			"passthrough_headers": ["X-Restaurant-Id", "X-Timezone"]
		}
	}`)
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("SECRET_KEY")
		os.Unsetenv("MICROSERVICES")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	require.Contains(t, cfg.Microservices, "reservas")
	svc := cfg.Microservices["reservas"]
	assert.Equal(t, []string{"X-Restaurant-Id", "X-Timezone"}, svc.PassthroughHeaders)
}

func TestServiceConfigPassthroughHeaders(t *testing.T) {
	svc := ServiceConfig{
		URL:                "http://localhost:4000",
		AllowedClients:     []string{"*"},
		PassthroughHeaders: []string{"X-Restaurant-Id", "X-Branch-ID"},
	}

	// 1. Cabeceras permitidas (evaluación case-insensitive)
	assert.True(t, svc.IsHeaderPassthroughAllowed("X-Restaurant-Id"))
	assert.True(t, svc.IsHeaderPassthroughAllowed("x-restaurant-id"))
	assert.True(t, svc.IsHeaderPassthroughAllowed("X-RESTAURANT-ID"))
	assert.True(t, svc.IsHeaderPassthroughAllowed("x-branch-id"))
	assert.True(t, svc.IsHeaderPassthroughAllowed("X-Branch-ID"))

	// 2. Cabeceras no permitidas
	assert.False(t, svc.IsHeaderPassthroughAllowed("X-Staff-Role"))
	assert.False(t, svc.IsHeaderPassthroughAllowed("X-Custom-Hack"))
	assert.False(t, svc.IsHeaderPassthroughAllowed(""))

	// 3. Microservicio sin passthrough_headers -> false para cualquier cabecera
	emptySvc := ServiceConfig{
		URL:            "http://localhost:4000",
		AllowedClients: []string{"*"},
	}
	assert.False(t, emptySvc.IsHeaderPassthroughAllowed("X-Restaurant-Id"))
}

func TestServiceConfigCaseInsensitiveRoles(t *testing.T) {
	// Requerido en minúscula ("user"), usuario con mayúscula ("USER")
	svcLower := ServiceConfig{
		RequiredRoles: []string{"user"},
	}
	assert.True(t, svcLower.IsRoleAllowed("USER"))
	assert.True(t, svcLower.IsRoleAllowed("user"))
	assert.True(t, svcLower.IsRoleAllowed("User"))
	assert.False(t, svcLower.IsRoleAllowed("admin"))

	// Requerido en mayúscula ("ADMIN"), usuario con minúscula ("admin")
	svcUpper := ServiceConfig{
		RequiredRoles: []string{"ADMIN"},
	}
	assert.True(t, svcUpper.IsRoleAllowed("admin"))
	assert.True(t, svcUpper.IsRoleAllowed("ADMIN"))
	assert.False(t, svcUpper.IsRoleAllowed("user"))
}


