#!/usr/bin/perl

# Perl Program to calculate Fibonancci
sub fib
{

# Retrieving the first argument
# passed with function calling
my $x = $_[0];

# checking if that value is 0 or 1
if ($x == 0 || $x == 1)
{
    return 1;
}

# Recursively calling function with the next value
# which is one less than current one
else
{
    if ($x == 20) {
       sleep 100;
    }
    return $x + fib($x - 1);
}
}

# Driver Code
$a = 30;

# Function call and printing result after return
print "Fibonancci of a number $a is ", fib($a), "\n";
