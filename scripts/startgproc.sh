#!/usr/bin/env bash

# simple gproc exerciser, starts gproc on the master node, lets
# the user optionally specify recompilation and pushing of the
# new gproc executables
MASTER=10.12.0.7 
IPPREF=10.12.0
RANGE="11-17"
DEBUG=0
LOC=/home/root
TIMESERVER=be.pool.ntp.org
PEERS=0

while getopts rgd:l:p:st: opt ; do
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
		l)
			LOC=$OPTARG
			;;
		s)
			TRACE='strace -f'
			;;
		p)
			PEERS=$OPTARG
			;;
		t)
			TIMESERVER=$OPTARG
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

expandrange()
{
	echo $1 | awk -F, -v 'pref='$IPPREF '
	{
		for(i = 1; i <= NF; i++){
			split($i, a, "-")
			for(j = a[1]; j <= a[2]; j++)
				print j
		}	
	}
	'
}

killgprocs()
{
	ssh -q root@$MASTER killall gproc 2>/dev/null >/dev/null
	for i in `expandrange $RANGE`; do
		ssh -q root@$IPPREF.$i killall gproc 2>/dev/null >/dev/null &
		ssh -q root@$IPPREF.$i rm -rf /tmp/xproc 2>/dev/null >/dev/null &
	done
	wait
}

killgprocs
trap "killgprocs;rm $SOCKNAME;exit 1" SIGHUP SIGINT SIGKILL SIGTERM SIGSTOP

OLDGOOS=`ssh root@$MASTER 'echo $GOOS'`
OLDGOARCH=`ssh root@$MASTER 'echo $GOARCH'`
OLDGOOS=linux
OLDGOARCH=arm
if [[ -n $RECOMPILEGOB ]]; then
	GOOS=linux
	GOARCH=arm
	(cd $GOROOT/src/pkg/gob && make clean >/dev/null && make install >/dev/null) || exit 1
	GOOS=$OLDGOOS
	GOARCH=$OLDGOARCH
	(cd $GOROOT/src/pkg/gob && make clean >/dev/null && make install >/dev/null) || exit 1
fi

# in a subshell to make sure we don't corrupt the working directory
if [[ -n $RECOMPILE ]]; then
	GOOS=linux
	GOARCH=arm
	(cd $GOROOT/src/cmd/gproc && make clean >/dev/null && make && file gproc) || exit 1
	
	for i in `expandrange $RANGE`; do
		scp gproc root@$IPPREF.$i:$LOC >/dev/null &
		ssh root@$IPPREF.$i ntpdate $TIMESERVER &
	done
	wait
	GOOS=$OLDGOOS
	GOARCH=$OLDGOARCH
	(cd $GOROOT/src/cmd/gproc && make clean && make install && file gproc) || exit 1
	scp gproc root@$MASTER:$LOC >/dev/null
fi

PORT=`ssh root@$MASTER rm $SRVADDR`

SOCKNAME=`ssh root@$MASTER mktemp /tmp/g.XXXXXX`
SRVADDR=/tmp/srvaddr

ssh root@$MASTER rm $SOCKNAME
ssh root@$MASTER $LOC/gproc  -npeers=$PEERS -debug=$DEBUG MASTER $SOCKNAME &

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
PORT=`ssh root@$MASTER cat $SRVADDR`
PORT=${PORT//*:/}
for i in `expandrange $RANGE`; do
	ssh root@$IPPREF.$i $TRACE $LOC/gproc -npeers=$PEERS -debug=$DEBUG  WORKER tcp4 $MASTER:$PORT $IPPREF.$i:$PORT &
done
sleep 1
if [[ ! -e /tmp/date ]]; then
	scp root@$IPPREF.`expandrange $RANGE | sed 1q`:/bin/date /tmp
fi
GRANGE=`expandrange $RANGE | wc -l | awk '{print "1-"$1}'`
#time ssh root@$MASTER gproc -npeers=$PEERS -debug=$DEBUG EXEC $SOCKNAME tcp $MASTER:$PORT $GRANGE /tmp/date
time ssh root@$MASTER $LOC/gproc -npeers=$PEERS -debug=$DEBUG EXEC $SOCKNAME tcp $MASTER:3463 $GRANGE /tmp/date
#time ssh root@$MASTER $LOC/gproc -npeers=$PEERS -debug=$DEBUG EXEC $SOCKNAME tcp $MASTER:3463 $GRANGE /tmp/date

#ssh $MASTER gproc -npeers=$PEERS -debug=$DEBUG EXEC $SOCKNAME tcp $MASTER:$PORT $GRANGE /tmp/date
# ssh $MASTER gproc -npeers=$PEERS -debug=$DEBUG EXEC $SOCKNAME tcp $MASTER:$PORT $GRANGE /tmp/date
# ssh $MASTER gproc -npeers=$PEERS -debug=$DEBUG EXEC $SOCKNAME tcp $MASTER:$PORT $GRANGE /tmp/date

# ssh $MASTER gproc -npeers=$PEERS -debug=$DEBUG EXEC $SOCKNAME tcp 0.0.0.0:0 $RANGE /tmp/date
# ssh $MASTER gproc -npeers=$PEERS -debug=$DEBUG EXEC $SOCKNAME tcp 0.0.0.0:0 $RANGE /tmp/date
ssh root@$MASTER rm $SOCKNAME