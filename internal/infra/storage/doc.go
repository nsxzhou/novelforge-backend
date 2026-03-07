// Package storage contains storage adapters that satisfy domain repository
// contracts.
//
// The current runtime ships with an in-memory provider so bootstrap, service,
// and handler wiring can depend on stable repository boundaries without a
// database dependency. Additional providers can be added behind the same
// factory and repository aggregation types.
package storage
