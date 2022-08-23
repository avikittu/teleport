package service

import (
	"io"
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

func validateConfig(cfg *Config) error {
	applyDefaults(cfg)

	if err := validateVersion(cfg); err != nil {
		return err
	}

	if err := verifyEnabledService(cfg); err != nil {
		return err
	}

	if err := validateAuthOrProxyServices(cfg); err != nil {
		return err
	}

	if cfg.DataDir == "" {
		return trace.BadParameter("config: please supply data directory")
	}

	for i := range cfg.Auth.Authorities {
		if err := services.ValidateCertAuthority(cfg.Auth.Authorities[i]); err != nil {
			return trace.Wrap(err)
		}
	}

	for _, tun := range cfg.ReverseTunnels {
		if err := services.ValidateReverseTunnel(tun); err != nil {
			return trace.Wrap(err)
		}
	}

	cfg.SSH.Namespace = types.ProcessNamespace(cfg.SSH.Namespace)

	return nil
}

func applyDefaults(cfg *Config) {
	if cfg.Version == "" {
		cfg.Version = defaults.TeleportConfigVersionV1
	}

	if cfg.Console == nil {
		cfg.Console = io.Discard
	}

	if cfg.Log == nil {
		cfg.Log = logrus.StandardLogger()
	}

	if cfg.PollingPeriod == 0 {
		cfg.PollingPeriod = defaults.LowResPollingPeriod
	}
}

func validateAuthOrProxyServices(cfg *Config) error {
	haveAuthServer := !cfg.AuthServer.IsEmpty()
	haveAuthServers := len(cfg.AuthServers) > 0
	haveProxyAddress := !cfg.ProxyAddress.IsEmpty()

	if cfg.Version == defaults.TeleportConfigVersionV3 {
		if haveAuthServers {
			return trace.BadParameter("config: auth_servers (string[]) has been changed to auth_server (string)")
		}

		if haveAuthServer && haveProxyAddress {
			return trace.BadParameter("config: cannot use both auth_server and proxy_address")
		}

		if !haveAuthServer && !haveProxyAddress {
			return trace.BadParameter("config: auth_server or proxy_address is required")
		}

		return nil
	}

	if haveAuthServer {
		return trace.BadParameter("config: auth_server is supported from config version v3 onwards")
	}

	if haveProxyAddress {
		return trace.BadParameter("config: proxy_address is supported from config version v3 onwards")
	}

	if !haveAuthServers {
		return trace.BadParameter("config: auth_servers is required")
	}

	return nil
}

func validateVersion(cfg *Config) error {
	hasVersion := utils.SliceContainsStr(defaults.TeleportVersions, cfg.Version)
	if !hasVersion {
		return trace.BadParameter("config: version must be one of %s", strings.Join(defaults.TeleportVersions, ", "))
	}

	return nil
}

func verifyEnabledService(cfg *Config) error {
	enabled := []bool{
		cfg.Auth.Enabled,
		cfg.SSH.Enabled,
		cfg.Proxy.Enabled,
		cfg.Kube.Enabled,
		cfg.Apps.Enabled,
		cfg.Databases.Enabled,
		cfg.WindowsDesktop.Enabled,
	}

	has := false
	for _, item := range enabled {
		if item {
			has = true

			break
		}
	}

	if !has {
		return trace.BadParameter(
			"config: enable at least one of auth_service, ssh_service, proxy_service, app_service, database_service, kubernetes_service or windows_desktop_service")
	}

	return nil
}
