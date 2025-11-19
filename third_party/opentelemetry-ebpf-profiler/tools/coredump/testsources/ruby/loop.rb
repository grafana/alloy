#!/usr/bin/env ruby

def is_prime(n)
  if n < 2
    return false
  elsif n == 2
    return true
  end

  ((2..(Math.sqrt(n)))).each do |i|
    return false if n % i == 0
  end
  return true
end

def sum_of_primes(n)
    sum_of_primes = 0
    x = 2
    while x < n
        if is_prime(x)
            sum_of_primes += x
        end
        x += 1
    end
    return sum_of_primes
end

loop do
  for i in 0..1000000 do
    puts i, sum_of_primes(i)
  end
end

