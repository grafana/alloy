<?php

function is_prime($number)
{
    // 1 is not prime
    if ( $number == 1 ) {
        return false;
    }
    // 2 is the only even prime number
    if ( $number == 2 ) {
        return true;
    }

    $x = sqrt($number);
    $x = floor($x);
    for ( $i = 2 ; $i <= $x ; ++$i ) {
        if ( $number % $i == 0 ) {
            break;
        }
    }
 
    if( $x == $i-1 ) {
        return true;
    } else {
        return false;
    }
}

while (1) {
	$start = 0;
	$end =   1000000;
	for($i = $start; $i <= $end; $i++)
	{
	    if(is_prime($i))
	    {
	        echo '<strong>'.$i.'</strong>, ';
	    }
	}
}

?>
