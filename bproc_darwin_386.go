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
	"net"
	"log"
	"syscall"
	"unsafe"
	"encoding/binary"
)

func tcpSockDial(Lserver string) int {
	/* try your best ... */
	Dprintf(2, "tcpSockDial: connect ", Lserver)
	a, err := net.ResolveTCPAddr(Lserver)
	if err != nil {
		Dprintf(2, "tcpSockDial: %s\n", err)
		return -1
	}
	sock, e := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
	if sock < 0 {
		Dprintf(2, "tcpSockDial: %v %v\n", sock, e)
		return -1
	}
	/* format: BE short family, short port, long addr */
	/* I'll do this bit stuffing until Go gets fixed and we can use a Conn for exec */
	addr := make([]byte, 16)
	addrlen := 16
	rawaddr := []byte(a.IP)
	addr[1] = syscall.AF_INET >> 8
	addr[0] = syscall.AF_INET
	addr[2] = uint8(a.Port >> 8)
	addr[3] = uint8(a.Port)

	addr[4] = uint8(rawaddr[12])
	addr[5] = uint8(rawaddr[13])
	addr[6] = uint8(rawaddr[14])
	addr[7] = uint8(rawaddr[15])
	//e = syscall.Connect(sock, a)
	_, _, e1 := syscall.Syscall(syscall.SYS_CONNECT, uintptr(sock), uintptr(unsafe.Pointer(&addr[0])), uintptr(addrlen))
	if e1 < 0 {
		if *DebugLevel > 2 {
			log.Printf("tcpSockDial: %v %v\n", sock, e)
		}
		return -1
	}
	Dprintf(2, "tcpSockDial: connnected %s\n", int(e1))
	return int(e1)

}

const (
	SIOCGIFCONF = 0xc00c6924
	SIOCGIFADDR = 0xc0206921
)
func getIfc() int {
	sock := tcpSockDial("74.125.87.99:80")
	if sock < 0 {
		Dprintf(2, "getIfc: %v\n", sock)
		return -1
	}
	ifc := make([]byte, 256)

	
	_,_,e1 := syscall.Syscall(syscall.SYS_IOCTL, uintptr(sock), uintptr(SIOCGIFADDR), uintptr(unsafe.Pointer(&ifc[0])))
	if e1 < 0 {
		Dprintf(2, "getIfc: ioctl: %v %v\n", sock, e1)
		return -1
	}
	log.Print(ifc)
	// so we are le.
	ifcbuf := make([]byte, 128)
	binary.LittleEndian.PutUint32(ifc, uint32(len(ifcbuf)))
	log.Print("pointers ",unsafe.Pointer(&ifc), " ", uintptr(unsafe.Pointer(&ifcbuf)))
	p := uintptr(unsafe.Pointer(&ifcbuf))
	ifc[4] = uint8(p)
	ifc[5] = uint8(p>>8)
	ifc[6] = uint8(p>>16)
	ifc[7] = uint8(p>>24)
	log.Printf("%x\n", binary.LittleEndian.Uint32(ifc[4:]))
	log.Print(ifc)
	
	_,_,e0 := syscall.Syscall(syscall.SYS_IOCTL, uintptr(sock), uintptr(SIOCGIFCONF), uintptr(unsafe.Pointer(&ifc)))
	if e0 < 0 {
		Dprintf(2, "getIfc: ioctl: %v %v\n", sock, e0)
		return -1
	}
	log.Print(ifc)
	log.Print(ifcbuf)
	return 0
}