<?php

function run_forever() {
    $max_size = 1000;
    $random_number_array = range(0, $max_size);
    shuffle($random_number_array);
    $size = rand(0, $max_size);
    $random_one = array_slice($random_number_array, 0, $size);
    shuffle($random_number_array);
    $random_two = array_slice($random_number_array, 0, $size);
    $sum = 0;
    for ($i = 0; $i < $size; $i++) {
          $sum += $random_one[$i] * $random_two[$i];
    }

    return;
}

while(1) {
	run_forever();
}
?>

