package main

/*

graphviz dumper for ad hoc trees.

will work with canvasviz eventually
*/

import (
	"io"
	"netchan"
)

func init() {
	roleFunc = graphVizRoleFunc
	onSendFunc = SendFunc
	onRecvFunc = RecvFunc
	onDialFunc = DialFunc
	onListenFunc = ListenFunc
	onAcceptFunc = AcceptFunc
	// set up a web browser
}

type graphNode struct {
	role string
	src  string
	dest string
}

var dotFileTemplate = template.MustParse(`
graph gproc {
	{{.repeated section graph}}
	{{src}} -> {{dest}}[label={{noderole}}]
	{{.end}}
}
`,
	nil)

var nodeRole string // the global role of this instance of gproc
func graphVizRoleFunc(role string) {
	var graph []graph
	nodeRole = role
	if role == "master" {
		// start http server
		// listen on channels and update the dot file
		t.SetDelims("{{", "}}")
		go func() {
			for {
				// receive on a netchan
				// then when you get it add it and execute
				// you also want to time out nodes.
				// how do you handle incoming?
				// you can do interesting stuff 
				node := <-nodeChan
				graph = append(graph, node)
				dotFileTemplate.Execute(graph)
			}
		}()
	} else {
		// just forward
		go func() {
			for _, c := range is {
				go func() {
					// select on the sending versus establishment
					nodeChan <- c
				}()
			}
		}()
	}
}

var (
	es map[string]chan graph
	is map[string]chan graph
)
// create a shadow ad hoc tree.
// the nodes percolate their nodes back up with netchans.
func SendFunc(funcname string, w io.Writer, arg interface{}) {
	// no op for now, can make the pretties later
}

func RecvFunc(funcname string, r io.Reader, arg interface{}) {
	// no op for now, can make the pretties later
}

func DialFunc(fam, laddr, raddr string) {
	e, err := netchan.Importer(fam, laddr)
	if err != nil {
		log.Exit("DialFunc: ", err)
	}
	is[raddr] = make(chan graph)
	e.Import("graphChan", is[raddr], netchan.Recv)
}

func ListenFunc(fam, laddr string) {
	e, err := netchan.Exporter(fam, laddr)
	if err != nil {
		log.Exit("ListenFunc: ", err)
	}
	es[laddr] = make(chan graph)
	e.Export("graphChan", es[laddr], netchan.Send)
	// import the send chan too later
}

func AcceptFunc(c net.Conn) {
	dest := c.RemoteAddr().String()
	src := c.LocalAddr().String()
	es[dest] <- &graph{role: nodeRole, src: src, dest: dest}
}
