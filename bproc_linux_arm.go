package main

import (
	"os"
	"net"
	"fmt"
	"syscall"
	"unsafe"
)

func connect(Lserver string) int {
	/* try your best ... */
	a, err := net.ResolveTCPAddr(Lserver)
	if err != nil {
		if *DebugLevel > 2 {
			fmt.Fprintf(os.Stderr, "%s\n", err)
		}
		return -1
	}
	sock, e := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
	if sock < 0 {
		if *DebugLevel > 2 {
			fmt.Printf("%v %v\n", sock, e)
		}
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
	_, _, e1 := syscall.Syscall(syscall.SYS_CONNECT, uintptr(sock), uintptr(unsafe.Pointer(&addr[0])), uintptr(addrlen))
	if e1 < 0 {
		if *DebugLevel > 2 {
			fmt.Printf("%v %v\n", sock, e)
		}
		return -1
	}
	return int(sock)

}


func ucred(fd int) (pid, uid, gid int) {
	var length [1]int
	creds := make([]int, 3)
	ucred := make([]uintptr, 6)
	length[0] = 12
	ucred[0] = syscall.SOL_SOCKET
	ucred[1] = syscall.SO_PEERCRED
	ucred[2] = uintptr(unsafe.Pointer(&creds[0]))
	ucred[3] = uintptr(unsafe.Pointer(&length[0]))
	_, _, e1 := syscall.Syscall(102, 15, uintptr(unsafe.Pointer(&ucred[0])), 0)

	if e1 < 0 {
		if *DebugLevel > 2 {
			fmt.Printf("%v %v\n", fd, e1)
		}
		return -1, -1, -1
	}
	return creds[0], creds[1], creds[2]
}

func unmount(path string) int {
	path8 := []byte(path)
	_, _, e1 := syscall.Syscall(syscall.SYS_UMOUNT, uintptr(unsafe.Pointer(&path8[0])), 0, 0)
	return int(e1)
}
