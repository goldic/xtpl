// Package webfuncs registers optional template functions when imported.
//
// A blank import registers randIcon, randColor and hash64 with package xtpl:
//
//	import _ "github.com/goldic/xtpl/webfuncs"
//
// randIcon and randColor select a value deterministically from IconsCollection
// and ColorsCollection using the supplied seed. hash64 returns the same value
// as Hash64. These functions are not suitable for cryptographic use.
//
// Import this package during program initialization, before concurrent template
// execution. Configure the collections before concurrent use.
package webfuncs
