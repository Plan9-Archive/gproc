/*

Gproc is a cluster process control and instantiation system. It creates an
ad hoc tree of worker and master processes and then runs commands on them,
downloading all of the necessary prerequisites into a private namespace
before it does so.

Run the master, then the slaves to join the master, then the commands.

Master: 
gproc m <unix-domain-socket>, e.g. gproc m /tmp/g

Note the socket number: 0.0.0.0:42092

start  slave: 
It has to run as root due to the private name space setup. 
sudo ./gproc s  tcp4 127.0.0.1:42092

Then run a command: 
./gproc  -f /etc/hosts,/tmp -localbin e  /tmp/x tcp 0.0.0.0:0 '1' /bin/ls -ld


and it should all work.
*/
package documentation