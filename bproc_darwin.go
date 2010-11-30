package main

import (
	"syscall"
	"unsafe"
	"log"
)

func privatemount(path string) int {
	log.Exit("privatemount called on OSX")
	return -1
}

func unshare() int {
	log.Exit("privatemount called on OSX")
	return -1
}

/* no on OSX? */
func ucred(fd int) (pid, uid, gid int) {
/*
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
		return -1,-1,-1
	}
	return creds[0], creds[1], creds[2]
 */
	return 0, 0, 0
}

func unmount(path string) int {
	path8 := []byte(path)
	_, _, e1 := syscall.Syscall(syscall.SYS_UNMOUNT, uintptr(unsafe.Pointer(&path8[0])), 0, 0)
	return int(e1)
}
