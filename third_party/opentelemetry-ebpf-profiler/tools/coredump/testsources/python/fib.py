#!/usr/bin/env python

def recur_fibo(n):
   if n <= 1:
       return n
   else:
       return(recur_fibo(n-1) + recur_fibo(n-2))

while True:
    print(recur_fibo(42))
