package main

import (
	"net"
	"syscall"
	"unsafe"
	"log"
)

var errors = []string{
        7:   "argument list too long",
        13:  "permission denied",
        98:  "address already in use",
        99:  "cannot assign requested address",
        68:  "advertise error",
        97:  "address family not supported by protocol",
        11:  "resource temporarily unavailable",
        114: "operation already in progress",
        52:  "invalid exchange",
        9:   "bad file descriptor",
        77:  "file descriptor in bad state",
        74:  "bad message",
        53:  "invalid request descriptor",
        56:  "invalid request code",
        57:  "invalid slot",
        59:  "bad font file format",
        16:  "device or resource busy",
        125: "operation canceled",
        10:  "no child processes",
        44:  "channel number out of range",
        70:  "communication error on send",
        103: "software caused connection abort",
        111: "connection refused",
        104: "connection reset by peer",
        35:  "resource deadlock avoided",
        89:  "destination address required",
        33:  "numerical argument out of domain",
        73:  "RFS specific error",
        122: "disk quota exceeded",
        17:  "file exists",
        14:  "bad address",
        27:  "file too large",
        112: "host is down",
        113: "no route to host",
        43:  "identifier removed",
        84:  "invalid or incomplete multibyte or wide character",
        115: "operation now in progress",
        4:   "interrupted system call",
        22:  "invalid argument",
        5:   "input/output error",
        106: "transport endpoint is already connected",
        21:  "is a directory",
        120: "is a named type file",
        127: "unknown error 127",
        129: "unknown error 129",
        128: "unknown error 128",
        51:  "level 2 halted",
        45:  "level 2 not synchronized",
        46:  "level 3 halted",
        47:  "level 3 reset",
        79:  "can not access a needed shared library",
        80:  "accessing a corrupted shared library",
        83:  "cannot exec a shared library directly",
        82:  "attempting to link in too many shared libraries",
        81:  ".lib section in a.out corrupted",
        48:  "link number out of range",
        40:  "too many levels of symbolic links",
        124: "wrong medium type",
        24:  "too many open files",
        31:  "too many links",
        90:  "message too long",
        72:  "multihop attempted",
        36:  "file name too long",
        119: "no XENIX semaphores available",
        100: "network is down",
        102: "network dropped connection on reset",
        101: "network is unreachable",
        23:  "too many open files in system",
        55:  "no anode",
        105: "no buffer space available",
        50:  "no CSI structure available",
        61:  "no data available",
        19:  "no such device",
        2:   "no such file or directory",
        8:   "exec format error",
        126: "unknown error 126",
        37:  "no locks available",
        67:  "link has been severed",
        123: "no medium found",
        12:  "cannot allocate memory",
        42:  "no message of desired type",
        64:  "machine is not on the network",
        65:  "package not installed",
        92:  "protocol not available",
        28:  "no space left on device",
        63:  "out of streams resources",
        60:  "device not a stream",
        38:  "function not implemented",
        15:  "block device required",
        107: "transport endpoint is not connected",
        20:  "not a directory",
        39:  "directory not empty",
        118: "not a XENIX named type file",
        131: "unknown error 131",
        88:  "socket operation on non-socket",
        95:  "operation not supported",
        25:  "inappropriate ioctl for device",
        76:  "name not unique on network",
        6:   "no such device or address",
        75:  "value too large for defined data type",
        130: "unknown error 130",
        1:   "operation not permitted",
        96:  "protocol family not supported",
        32:  "broken pipe",
        71:  "protocol error",
        93:  "protocol not supported",
        91:  "protocol wrong type for socket",
        34:  "numerical result out of range",
        78:  "remote address changed",
        66:  "object is remote",
        121: "remote I/O error",
        85:  "interrupted system call should be restarted",
        30:  "read-only file system",
        108: "cannot send after transport endpoint shutdown",
        94:  "socket type not supported",
        29:  "illegal seek",
        3:   "no such process",
        69:  "srmount error",
        116: "stale NFS file handle",
        86:  "streams pipe error",
        62:  "timer expired",
        110: "connection timed out",
        109: "too many references: cannot splice",
        26:  "text file busy",
        117: "structure needs cleaning",
        49:  "protocol driver not attached",
        87:  "too many users",
        18:  "invalid cross-device link",
        54:  "exchange full",
}

func tcpSockDial(Lserver string) int {
	Dprint(2, "tcpSockDial: connect ", Lserver)
	a, err := net.ResolveTCPAddr(Lserver)
	if err != nil {
		Dprintf(2, "tcpSockDial: ResolveTCPAddr: %s\n", err)
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
	_, _, e1 := syscall.Syscall(syscall.SYS_CONNECT, uintptr(sock), uintptr(unsafe.Pointer(&addr[0])), uintptr(addrlen))
	if e1 != 0 {
		Dprintf(2, "tcpSockDial: connect: failed %v %v\n", sock, errors[e1])
		return -1
	}
	Dprint(2, "tcpSockDial: connnected ", int(e1))
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
			log.Printf("%v %v\n", fd, e1)
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
