//go:build !windows

package sessioncache

import "errors"

func protect([]byte) ([]byte, error) {
	return nil, errors.New("sifreli oturum onbellegi su anda yalnizca Windows'ta destekleniyor")
}

func unprotect([]byte) ([]byte, error) {
	return nil, errors.New("sifreli oturum onbellegi su anda yalnizca Windows'ta destekleniyor")
}
