#!/bin/bash
# syntax: copytokane.sh start end gproc-executable
for i in `seq $1 $2` 
do
echo $i
ssh -o StrictHostKeyChecking=no -q root@kn$i  'rm gproc; mkdir -p /lib64'&
done
wait

for i in `seq $1 $2` 
do
echo $i
scp /lib/ld-* root@kn$i:/lib &
scp /lib64/ld-* root@kn$i:/lib64 &
#scp /lib32/libc.so.6 /lib32/libpthread.so.0 root@kn$i:/lib32&
scp -o StrictHostKeyChecking=no $3 root@kn$i: &
done
wait

