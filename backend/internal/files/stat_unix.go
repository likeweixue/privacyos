//go:build linux || darwin

package files

import "syscall"

// statT aliases the platform stat structure used to resolve file owners.
type statT = syscall.Stat_t
