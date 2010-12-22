/*
 * gproc, a Go reimplementation of the LANL version of bproc and the LANL XCPU software. 
 * 
 * This software is released under the Lesser Gnu Programming License, incorporated herein by reference. 
 *
 * Copyright (2010) Sandia Corporation. Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation, 
 * the U.S. Government retains certain rights in this software.
 */

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
