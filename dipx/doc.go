// Package dipx implements the .dipx distributable bundle format for Dippin
// workflows. See docs/superpowers/specs/2026-05-06-dipx-bundle-format-design.md
// for the normative specification.
//
// The package emits no log output; all observability is via returned errors
// (use errors.Is for sentinels and errors.As to extract structured fields
// from *BundleError).
package dipx
