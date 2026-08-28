package config

import (
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultAPIURL = "https://app.usetix.io"
	APIURLEnv     = "USETIX_API_URL"
	ConfigDirEnv  = "USETIX_CONFIG_DIR"
	ProfileEnv    = "USETIX_PROFILE"
	TokenEnv      = "USETIX_TOKEN"
)

type GetenvFunc func(string) string

func Dir(getenv GetenvFunc) (string, error) {
	if directory := strings.TrimSpace(getenv(ConfigDirEnv)); directory != "" {
		return filepath.Clean(directory), nil
	}

	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "usetix"), nil
}

func File(directory string) string {
	return filepath.Join(directory, "config.json")
}

func ResolveAPIURL(flagValue, envValue, profileValue string) string {
	for _, value := range []string{flagValue, envValue, profileValue, DefaultAPIURL} {
		if value = strings.TrimSpace(value); value != "" {
			return strings.TrimRight(value, "/")
		}
	}
	return DefaultAPIURL
}

func ValidateAPIURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("API URL must use http or https")
	}
	if parsed.Host == "" {
		return errors.New("API URL must include a host")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("API URL must not include credentials, a query, or a fragment")
	}
	if parsed.Scheme == "http" && !isLocalhost(parsed.Hostname()) {
		return errors.New("API URL must use https unless it targets localhost")
	}
	return nil
}

func isLocalhost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}
