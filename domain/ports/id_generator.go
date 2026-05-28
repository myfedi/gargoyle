package ports

// IDGenerator creates application-level opaque identifiers for use cases that
// need to assign IDs before calling lower-level persistence ports.
type IDGenerator interface {
	NewID() (string, error)
}
