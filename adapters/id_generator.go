package adapters

import "github.com/myfedi/gargoyle/infrastructure/db"

// ULIDGenerator adapts the infrastructure ULID helper to the domain ID port.
type ULIDGenerator struct{}

func NewULIDGenerator() ULIDGenerator { return ULIDGenerator{} }

func (ULIDGenerator) NewID() (string, error) { return db.NewULID() }
