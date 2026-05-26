// Package example demonstrates the sqld-gen-bob ORM integration: bob models that
// share sqld-gen-go's leaf types, run on one *pgxpool.Pool, and bridge to the
// flat sqld models via ToSqld. It is a separate module so the bob dependency
// stays out of the root go.mod.
package example
