// Package go_diia provides a Go SDK for Diia Document Sharing.
//
// The root client exposes small service namespaces for authentication,
// branches, offers, sharing requests, statuses, and multipart callbacks.
// All consumer-facing models and constructors are re-exported from this root
// package while their implementations are grouped by concern internally.
// Native IIT cryptography is kept outside the core module behind
// CryptoProvider.
package go_diia
