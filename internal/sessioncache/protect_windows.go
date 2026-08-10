//go:build windows

package sessioncache

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

func protect(plaintext []byte) ([]byte, error) {
	return cryptProtect(plaintext, true)
}

func unprotect(ciphertext []byte) ([]byte, error) {
	return cryptProtect(ciphertext, false)
}

func cryptProtect(input []byte, encrypt bool) ([]byte, error) {
	if len(input) == 0 {
		return nil, errors.New("korunacak veri bos")
	}
	in := windows.DataBlob{Size: uint32(len(input)), Data: &input[0]}
	var out windows.DataBlob
	var err error
	if encrypt {
		err = windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	}()
	result := make([]byte, int(out.Size))
	copy(result, unsafe.Slice(out.Data, int(out.Size)))
	return result, nil
}
