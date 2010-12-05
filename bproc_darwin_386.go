package main

import (
	"net"
	"log"
	"syscall"
	"unsafe"
)

func connect(Lserver string) int {
	/* try your best ... */
	Dprintf(2, "connect: connecting ", Lserver)
	a, err := net.ResolveTCPAddr(Lserver)
	if err != nil {
		Dprintf(2, "connect: %s\n", err)
		return -1
	}
	sock, e := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
	if sock < 0 {
		Dprintf(2, "connect: %v %v\n", sock, e)
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
			log.Printf("connect: %v %v\n", sock, e)
		}
		return -1
	}
	Dprintf(2, "connect: connnected %s\n", int(e1))
	return int(e1)

}
