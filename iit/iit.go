// Package iit provides the optional IIT crypto provider for Diia callbacks.
package iit

import (
	diia "github.com/stremovskyy/go-diia"
	implementation "github.com/stremovskyy/go-diia-iit"
)

type (
	Config               = implementation.Config
	FileConfig           = implementation.FileConfig
	ConfigLoadBuilder    = implementation.ConfigLoadBuilder
	Provider             = implementation.Provider
	ProviderBuilder      = implementation.ProviderBuilder
	ProviderCloseBuilder = implementation.ProviderCloseBuilder
	IITError             = implementation.IITError
)

var _ diia.CryptoProvider = (*Provider)(nil)

var (
	ErrValidation         = implementation.ErrValidation
	ErrInitialize         = implementation.ErrInitialize
	ErrCertificate        = implementation.ErrCertificate
	ErrPrivateKey         = implementation.ErrPrivateKey
	ErrDecrypt            = implementation.ErrDecrypt
	ErrVerify             = implementation.ErrVerify
	ErrFinalize           = implementation.ErrFinalize
	ErrClosed             = implementation.ErrClosed
	ErrAlreadyInitialized = implementation.ErrAlreadyInitialized
)

func NewConfig() *Config { return implementation.NewConfig() }

func NewFileConfig() *FileConfig { return implementation.NewFileConfig() }

func LoadConfig() ConfigLoadBuilder { return implementation.LoadConfig() }

func NewProvider() ProviderBuilder { return implementation.NewProvider() }
