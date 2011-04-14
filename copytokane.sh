#!/bin/bash
for i in `seq $1 $2` 
do
echo $i
ssh -o StrictHostKeyChecking=no,ConnectTimeout=1 -q root@kn$i  rm -f gproc_linux_386 &
done
wait

for i in `seq $1 $2` 
do
echo $i
scp /lib/ld-* root@kn$i:/lib &
scp gproc_linux_386  root@kn$i: &
done
wait

