package runtime

import (
	"fmt"
	"os"
	"strings"
)

type ServiceEnvOptions struct {
	ServiceName         string
	DefaultListenAddr   string
	ListenAddrEnvKey    string
	RequiredNonLocalEnv []string
}

type ServiceEnvConfig struct {
	Environment    string
	ListenAddr     string
	ServiceVersion string
}

func LoadServiceEnvConfig(getenv func(string) string, options ServiceEnvOptions) (ServiceEnvConfig, error) {
	if getenv == nil {
		return ServiceEnvConfig{}, fmt.Errorf("getenv is required")
	}
	serviceName := strings.TrimSpace(options.ServiceName)
	if serviceName == "" {
		return ServiceEnvConfig{}, fmt.Errorf("service name is required")
	}

	env := normalizeEnv(getenv("BREVIO_ENV"))
	listenEnvKey := strings.TrimSpace(options.ListenAddrEnvKey)
	if listenEnvKey == "" {
		listenEnvKey = strings.ToUpper(strings.ReplaceAll(serviceName, "-", "_")) + "_LISTEN_ADDR"
	}
	listenAddr := strings.TrimSpace(getenv(listenEnvKey))
	if listenAddr == "" {
		listenAddr = strings.TrimSpace(options.DefaultListenAddr)
	}
	if listenAddr == "" {
		return ServiceEnvConfig{}, fmt.Errorf("listen address is required for service %s", serviceName)
	}

	serviceVersion := strings.TrimSpace(getenv("SERVICE_VERSION"))
	if serviceVersion == "" {
		serviceVersion = "0.1.0"
	}

	if !isLocalLikeEnv(env) {
		for _, key := range options.RequiredNonLocalEnv {
			if strings.TrimSpace(getenv(key)) == "" {
				return ServiceEnvConfig{}, fmt.Errorf("%s is required when BREVIO_ENV=%s for %s", key, env, serviceName)
			}
		}
	}

	return ServiceEnvConfig{
		Environment:    env,
		ListenAddr:     listenAddr,
		ServiceVersion: serviceVersion,
	}, nil
}

func ResolveSecretWithLocalDefault(getenv func(string) string, key, env, localDefault string) (string, error) {
	if getenv == nil {
		return "", fmt.Errorf("getenv is required")
	}
	secret := strings.TrimSpace(getenv(key))
	if secret != "" {
		return secret, nil
	}
	if isLocalLikeEnv(env) {
		return localDefault, nil
	}
	return "", fmt.Errorf("%s is required when BREVIO_ENV=%s", key, env)
}

func EnvStatus(key string) string {
	if strings.TrimSpace(os.Getenv(key)) == "" {
		return "not_configured"
	}
	return "configured"
}

func normalizeEnv(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "local"
	}
	return trimmed
}

func isLocalLikeEnv(env string) bool {
	switch normalizeEnv(env) {
	case "local", "dev", "test":
		return true
	default:
		return false
	}
}
