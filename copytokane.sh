#!/bin/bash
for i in `seq $1 $2` 
do
echo $i
ssh -o StrictHostKeyChecking=no -q root@kn$i  rm gproc_linux_amd64 &
done
wait

for i in `seq $1 $2` 
do
echo $i
scp /lib/ld-* root@kn$i:/lib &
scp -o StrictHostKeyChecking=no gproc_linux_amd64  root@kn$i: &
done
wait

