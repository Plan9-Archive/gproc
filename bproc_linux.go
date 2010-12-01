package main

import (
	"syscall"
	"unsafe"
)

const (
	linuxhack = 0xc0ed0000
)

func privatemount(pathbase string) int {
	path := []byte(pathbase)
	none := []byte("none")
	tmpfs := []byte("tmpfs")
	_, _, syscallerr := syscall.Syscall6(syscall.SYS_MOUNT,
		uintptr(unsafe.Pointer(&none[0])),
		uintptr(unsafe.Pointer(&path[0])),
		uintptr(unsafe.Pointer(&tmpfs[0])),
		uintptr(linuxhack),
		uintptr(0),
		uintptr(0))
	return int(syscallerr)
}

func unshare() int {
	_, _, syscallerr := syscall.Syscall(syscall.SYS_UNSHARE, uintptr(0x00020000), uintptr(0), uintptr(0))
	return int(syscallerr)
}
