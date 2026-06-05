package clientapi

// InstanceInfo returns static server metadata used by client-compatible
// clients during startup.
func (u Instance) InstanceInfo() InstanceInfo {
	return InstanceInfo{Host: u.deps.Host, Domain: u.deps.Domain, Title: "Gargoyle", Description: "Gargoyle federated server", ServerVersion: u.deps.ServerVersion}
}
