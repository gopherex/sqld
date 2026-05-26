package pocshared

import "github.com/jackc/pgx/v5/pgtype"

// GeoPoint is the shared, sqld-controlled Go type for a Postgres composite type
//
//	CREATE TYPE geo_point AS (lat double precision, lng double precision);
//
// It stands in for what sqld-gen-go would emit for a composite column: a single
// canonical Go struct that BOTH bob's ORM args and sqld's raw query structs
// reference. To round-trip a composite through pgx, the type implements pgx's
// composite getter/scanner interfaces (pgtype.CompositeIndexGetter /
// pgtype.CompositeIndexScanner) — the registration-required path, as opposed to
// the string-kind enum which needs none.
type GeoPoint struct {
	Lat float64
	Lng float64
}

// Compile-time proof GeoPoint satisfies pgx's composite interfaces.
var (
	_ pgtype.CompositeIndexGetter  = GeoPoint{}
	_ pgtype.CompositeIndexScanner = (*GeoPoint)(nil)
)

// IsNull implements pgtype.CompositeIndexGetter. A value GeoPoint is never NULL.
func (GeoPoint) IsNull() bool { return false }

// Index implements pgtype.CompositeIndexGetter: returns field i for encoding.
func (g GeoPoint) Index(i int) any {
	switch i {
	case 0:
		return g.Lat
	case 1:
		return g.Lng
	default:
		return nil
	}
}

// ScanNull implements pgtype.CompositeIndexScanner.
func (g *GeoPoint) ScanNull() error {
	*g = GeoPoint{}
	return nil
}

// ScanIndex implements pgtype.CompositeIndexScanner: returns a scan target for i.
func (g *GeoPoint) ScanIndex(i int) any {
	switch i {
	case 0:
		return &g.Lat
	case 1:
		return &g.Lng
	default:
		return nil
	}
}
