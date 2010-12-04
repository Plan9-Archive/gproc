package main


/* let's be nice and do an Ldd on each file. That's helpful to people. Later. */
func buildcmds(file, root, libs string) []Acmd {
	e, _ := ldd.Ldd(file, root, libs)
	/* now we have a list of file names. From this, we create the in-memory
	 * packed set of files/symlinks/directory descriptions. We also need to track
	 * what weve made and might have made earlier, to avoid duplicates.
	 */
	cmds := make([]Acmd, len(e))
	for i, s := range e {
		cmds[i].name = s
		cmds[i].fullpathname = root + s
		fi, _ := os.Stat(root + s)
		cmds[i].fi = *fi
	}
	return cmds
}

func netwaiter(fam, server string, nw int, c net.Conn) (chan int, Listener) {
	workers := make(chan int, nw)
	l, err := Listen(fam, server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Listen: %s\n", err)
		return nil, nil
	}

	go func() {
		for ; nw > 0; nw-- {
			conn, err := l.Accept()
			if err != nil {
				log.Printf("%s\n", err)
				continue
			}
			go netrelay(conn, workers, c)
		}
	}()
	return workers, l
}

func netrelay(c net.Conn, workers chan int, client net.Conn) {
	data := make([]byte, 1024)
	for {
		n, _ := c.Read(data)
		if n <= 0 {
			break
		}
		amt, err := client.Write(data)
		if amt <= 0 {
			log.Printf("Write failed: amt %d, err %v\n", amt, err)
			break
		}
		if err != nil {
			log.Printf("Write failed: %v\n", err)
			break
		}
	}
	workers <- 1
}

func readitin(s, root string) ([]byte, os.FileInfo, os.Error) {
	fi, _ := os.Stat(root + s)
	f, _ := os.Open(s, os.O_RDONLY, 0)
	bytes := make([]byte, fi.Size)
	f.Read(bytes)
	return bytes, *fi, nil
}

type Arg struct {
	Msg []byte
}

func Ping(arg *Arg, res *Res) os.Error {
	res.Msg = arg.Msg
	return nil
}

func Debug(arg *SetDebugLevel, res *SetDebugLevel) os.Error {
	res.level = *DebugLevel
	*DebugLevel = arg.level
	return nil
}
