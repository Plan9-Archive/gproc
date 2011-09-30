#!/bin/bash
for i in `seq $1 $2` 
do
echo $i
ssh -o StrictHostKeyChecking=no -q root@kn$i  rm gproc_linux_amd64 gproc_linux_386&
done
wait

for i in `seq $1 $2` 
do
echo $i
scp /lib/ld-* root@kn$i:/lib &
scp /lib32/ld-* root@kn$i:/lib32 &
scp /lib32/libc.so.6 /lib32/libpthread.so.0 root@kn$i:/lib32&
scp -o StrictHostKeyChecking=no gproc_linux_amd64  gproc_linux_386 root@kn$i: &
done
wait

