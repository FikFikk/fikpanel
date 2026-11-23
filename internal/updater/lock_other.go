//go:build !linux

package updater

import "errors"

// acquireLock tidak didukung di luar Linux karena pembaruan mandiri hanya ditujukan untuk server Linux.
func acquireLock(string) (func(), error) {
	return nil, errors.New("pembaruan mandiri FikPanel hanya didukung di Linux")
}
