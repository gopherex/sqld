// Package nodeid provides a deterministic, immutable AST node-id path builder.
// Each Builder holds an immutable path string; Child and Index return new
// Builders so callers can never mutate shared state.
package nodeid

import "strconv"

// Builder holds an immutable path string identifying a specific AST node.
type Builder struct {
	path string
}

// New creates a root Builder whose path is root.
func New(root string) Builder {
	return Builder{path: root}
}

// Child returns a new Builder with path b.path + "/" + name.
func (b Builder) Child(name string) Builder {
	return Builder{path: b.path + "/" + name}
}

// Index returns a new Builder with path b.path + ":" + i.
func (b Builder) Index(i int) Builder {
	return Builder{path: b.path + ":" + strconv.Itoa(i)}
}

// String returns the path string.
func (b Builder) String() string {
	return b.path
}
