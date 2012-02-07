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
	"encoding/binary"
	"syscall"
	"unsafe"
)

func getIfc() int {
	sock := tcpSockDial("74.125.87.99:80")
	if sock < 0 {
		log_info("getIfc: %v\n", sock)
		return -1
	}
	ifc := make([]byte, 256)

	_, _, e1 := syscall.Syscall(syscall.SYS_IOCTL, uintptr(sock), uintptr(SIOCGIFADDR), uintptr(unsafe.Pointer(&ifc[0])))
	if e1 < 0 {
		log_info("getIfc: ioctl: %v %v\n", sock, e1)
		return -1
	}
	log_info(ifc)
	// so we are le.
	ifcbuf := make([]byte, 128)
	binary.LittleEndian.PutUint32(ifc, uint32(len(ifcbuf)))
	log_info("pointers ", unsafe.Pointer(&ifc), " ", uintptr(unsafe.Pointer(&ifcbuf)))
	p := uintptr(unsafe.Pointer(&ifcbuf))
	ifc[4] = uint8(p)
	ifc[5] = uint8(p >> 8)
	ifc[6] = uint8(p >> 16)
	ifc[7] = uint8(p >> 24)
	log_info("%x\n", binary.LittleEndian.Uint32(ifc[4:]))
	log_info(ifc)

	_, _, e0 := syscall.Syscall(syscall.SYS_IOCTL, uintptr(sock), uintptr(SIOCGIFCONF), uintptr(unsafe.Pointer(&ifc)))
	if e0 < 0 {
		log_info("getIfc: ioctl: %v %v\n", sock, e0)
		return -1
	}
	log_info(ifc)
	log_info(ifcbuf)
	return 0
}
