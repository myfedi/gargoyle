package mastodon

// InstanceInfo returns static server metadata used by Mastodon-compatible
// clients during startup.
func (u UseCase) InstanceInfo() InstanceInfo {
	return InstanceInfo{Host: u.cfg.Host, Domain: u.cfg.Domain, Title: "Gargoyle", Description: "Gargoyle federated server", ServerVersion: u.cfg.ServerVersion}
}
