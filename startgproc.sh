#!/usr/bin/env bash

# simple gproc exerciser, starts gproc on the master node, lets
# the user optionally specify recompilation and pushing of the
# new gproc executables
MASTER=10.12.0.7 
IPPREF=10.12.0
RANGE="11 17"
DEBUG=0

while getopts rgd: opt ; do
	case "$opt" in
		r) 
			RECOMPILE=1
			;;
		g) 
			RECOMPILEGOB=1
			RECOMPILE=1
			;;
		d) 
			DEBUG=$OPTARG
			;;
		\\?) 
			echo "Error: unknown flag" >&2 
			;;
	esac
done

shift `expr $OPTIND - 1`

case $# in
	1)
		MASTER=$1
		IPPREF=${MASTER%%.*%}
		;;
	2)
		MASTER=$1
		IPPREF=$2
		;;
	3)
		MASTER=$1
		IPPREF=$2
		RANGE=$3
		;;		
esac

killgprocs()
{
	ssh -q $MASTER killall gproc 2>/dev/null >/dev/null
	for i in `seq $RANGE`; do
		ssh -q root@$IPPREF.$i killall gproc 2>/dev/null >/dev/null &
	done
	wait
}

killgprocs
trap "killgprocs;exit 1" SIGHUP SIGINT SIGKILL SIGTERM SIGSTOP

GOOS=darwin
GOARCH=386
if [[ -n $RECOMPILEGOB ]]; then
	GOOS=linux
	GOARCH=arm
	make clean >/dev/null && make install >/dev/null || exit 1
	GOOS=darwin
	GOARCH=386
	(cd $GOROOT/src/pkg/gob && make install >/dev/null) || exit 1
fi

# in a subshell to make sure we don't corrupt the working directory
if [[ -n $RECOMPILE ]]; then
	(cd $GOROOT/src/cmd/gproc && make install >/dev/null) || exit 1
	scp gproc $MASTER:$GOROOT/bin >/dev/null
	GOOS=linux
	GOARCH=arm
	(cd $GOROOT/src/cmd/gproc && make clean >/dev/null && make >/dev/null) || exit 1
	for i in `seq $RANGE`; do
		scp gproc root@$IPPREF.$i:/home/root/ >/dev/null &
	done
	wait
fi

SOCKNAME=`mktemp /tmp/g.XXXXXX`
SRVADDR=/tmp/srvaddr

rm $SOCKNAME
ssh $MASTER gproc  -debug=$DEBUG MASTER $SOCKNAME &

# hg pull http://bitbucket.org/npe/gproc # should be goinstall -c bitbucket.org/npe/gproc
# for i in `seq 11 17`; do
# 	scp /root/go/src/cmd/gproc/gproc 10.12.0.$i:/usr/bin &
# done

# this means that gproc needs to restart itself and run itself, ssh is silly in this environment.
# another thing that gproc should do it know its role based on something like ndb. Right now you need to think too hard about it, is this what the json file will be for?
# get linux to push itself
# separate out gproc into a package then you can write commands that provide arbitrary interface
# is there a scalable way for the master to advertise itself?

sleep 1
PORT=`cat $SRVADDR`
PORT=${PORT//*:/}
for i in `seq $RANGE`; do
	ssh root@$IPPREF.$i /home/root/gproc -debug=$DEBUG  WORKER tcp4 $MASTER:$PORT &
done
sleep 1
if [[ ! -e /tmp/date ]]; then
	scp root@$IPPREF.${RANGE// */}:/bin/date /tmp
fi
GRANGE=`echo $RANGE | awk '{print 1"-"($2-$1+1)}'`
ssh $MASTER gproc -debug=$DEBUG EXEC $SOCKNAME tcp 0.0.0.0:0 $GRANGE /tmp/date

