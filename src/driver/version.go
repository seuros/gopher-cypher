package driver

import "github.com/seuros/gopher-cypher/src/internal/boltutil"

// Version returns the current version of the gopher-cypher driver
func Version() string {
	return boltutil.LibraryVersion
}

// UserAgent returns the user agent string used in Bolt protocol communications
func UserAgent() string {
	return "gopher-cypher::Bolt/" + boltutil.LibraryVersion
}