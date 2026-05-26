// Package pocshared stands in for the canonical Go types that sqld's own
// Go code generator (sqld-gen-go) would emit. The PoC proves that bob can be
// forced to reuse THESE types for its ORM models instead of generating its own
// parallel enum type — the type-unification linchpin of the ORM⊕sqlc symbiosis.
package pocshared

// AccountStatus is the shared, sqld-controlled Go type for the Postgres enum
// `account_status`. Both bob's generated model and sqld's query structs are
// meant to reference this single canonical type.
type AccountStatus string

const (
	AccountStatusActive    AccountStatus = "active"
	AccountStatusSuspended AccountStatus = "suspended"
	AccountStatusClosed    AccountStatus = "closed"
)
