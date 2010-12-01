#!/usr/bin/env bash

simple gproc exerciser, starts gprocs on 
MASTER=10.12.0.7
IPPREF=10.12.0
RANGE="11 17"

case $# in
	1 )
		MASTER=$1
		IPPREF=${MASTER%%.*%}
		;;
	2 )
		MASTER=$1
		IPPREF=$2
		;;
	3 )
		MASTER=$1
		IPPREF=$2
		RANGE=$3
		;;		
esac

ssh $MASTER killall gproc 2>/dev/null
SOCKNAME=`mktemp /tmp/g.XXXXXX`
SRVADDR=`mktemp /tmp/servaddr.XXXXXX`

rm $SOCKNAME
$(ssh $MASTER gproc master $SOCKNAME | grep 'Serving on' | sed 's/Serving on 0.0.0.0://g' >$SRVADDR) &

# hg pull http://bitbucket.org/npe/gproc # should be goinstall -c bitbucket.org/npe/gproc
# for i in `seq 11 17`; do
# 	scp /root/go/src/cmd/gproc/gproc 10.12.0.$i:/usr/bin &
# done

# this means that gproc needs to restart itself and run itself, ssh is silly in this environment.
# another thing that gproc should do it know its role based on something like ndb. Right now you need to think too hard about it, is this what the json file will be for?
# get linux to push itself
# separate out gproc into a package then you can write commands that provide arbitrary interface
# is there a scalable way for the master to advertise itself?

for i in `seq $RANGE`; do
	ssh root@$IPPREF.$i killall gproc 2>/dev/null
done

for i in `seq $RANGE`; do
	ssh root@$IPPREF.$i gproc newworker tcp4 $MASTER:`cat $SRVDADDR` &
done
sleep 20
ssh $MASTER gproc exec $SOCKNAME tcp 0.0.0.0:0 ${RANGE// /-} /tmp/date
