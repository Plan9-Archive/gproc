/*
 * gproc, a Go reimplementation of the LANL version of bproc and the LANL XCPU software. 
 * 
 * This software is released under the Lesser Gnu Programming License, incorporated herein by reference. 
 *
 * Copyright (2010) Sandia Corporation. Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation, 
 * the U.S. Government retains certain rights in this software.
 */

package main

type nodeRange struct {
	beg int
	end int
}

func newNodeRange(i int) *nodeRange {
	return &nodeRange{beg: i, end: i}
}

func (n *nodeRange) String() string {
	if n.beg == n.end {
		return string(n.beg)
	}
	return string(n.beg) + "-" + string(n.end)
}


type rangeList struct {
	l list.List
}

func newRangeList() (rl rangeList) {
	rl.l = list.New()
	return
}

func (rl *rangeList) Add(i int) {
	for e := rl.l.Front(); e != nil; e = e.Next() {
		switch r := e.Value.(*nodeRange); {
		case r.beg < i && i < r.end:
			return
		case i < r.beg:
			rl.l.InsertBefore(e, newNodeRange(i))
			return
		}
	}
	rl.l.PushBack(newNodeRange(i))
}

func (rl *rangeList) String() (s string) {
	var ss []string
	for e := rl.l.Front(); e != nil; e = e.Next() {
		ss = append(ss, e.Value.(*nodeRange).String())
	}
	return strings.Join(ss, ",")
}


type mergeFifoLine struct {
	r rangeList
	s string
}

func (m *mergeFifoLine) String() string {

	return rs.String() + " " + m.s
}

type mergeReadWriter struct {
	w           bufio.Writer
	r           bufio.Reader
	fl          []mergeFifoLine
	alreadyRead map[string]*mergeFifoLine
}

func NewMergeReadWriter(r *io.Reader, w *io.Writer) (m mergeReadWriter) {
	m.r = bufio.NewReader(r)
	m.w = bufio.NewWriter(w)
	return
}

/*

this guy is highly concurrent.  
lots of people are going to be reading and writing him. 
right now I'm handling serialization by having individual io proxies that just read and write to stdout hierarchically. 



*/

func (m *mergeReadWriter) Write(p []byte) (n int, err os.Error) {
	// I need a bytes buffer/bufio that I can drain. 
	b := bytes.NewBuffer(p)
	for {
		s, err := b.ReadString('\n')
		if err != nil {
			return
		}
		/*
			where to get nodeNum

		*/
		if l, found := alreadyRead[s]; found {
			l.r.Add(nodeNum)
			continue
		}
		nr := newRangeList()
		nr.Add(i)
		fl := &mergeFifoLine{nr, s}
		alreadyRead[s] = fl
		v = append(v, fl)
	}
	// what to do about left overs?
}

func (m *mergeReadWriter) Read(p []byte) (n int, err os.Error) {
	if len(m.fl) > 10 {
		for _, l := range mv.fl {
			m.rw.WriteString(l.String())
		}
		m.alreadyRead = make(map[string]*mergeFifoLine)
	}
	// this is going to be weird, this is not going to be blocking. 
	n, err = m.rw.Read(p)
	return
}

// ReadFrom reads data from r until EOF and appends it to the buffer.
// The return value n is the number of bytes read.
// Any error except os.EOF encountered during the read
// is also returned.
func (mw *mergeReadWriter) ReadFrom(r io.Reader) (n int64, err os.Error) {
	if mr, ok := r.(*mergeReadWriter); ok {
		mw.fl = append(mw.fl, mr.fl)
		// how do I deal with the fact that I'm not aware of how much I read?
		return
	}
	if rpc, ok := r.(*RpcClientServer); ok {
		newfl := make([]*mergeFifoLine)
		rpc.Recv(newfl)
		mw.fl = append(mw.fl, newfl)
		// how do I deal with the fact that I'm not aware of how much I read?
		return
	}
	n, err = io.Copy(mw, r)
	return
}

// WriteTo writes data to w until the buffer is drained or an error
// occurs. The return value n is the number of bytes written.
// Any error encountered during the write is also returned.
func (mr *mergeReadWriter) WriteTo(w io.Writer) (n int64, err os.Error) {
	if mw, ok := w.(*mergeReadWriter); ok {
		n, err = mw.ReadFrom(mr)
		return
	}
	if rpc, ok := w.(*RpcClientServer); ok {
		rpc.Send(mr.fl)
		// how do I deal with the fact that I'm not aware of how much I read?
		return
	}
	n, err = io.Copy(w, mr)
	return
}
