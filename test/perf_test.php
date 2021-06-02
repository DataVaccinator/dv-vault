<?php
/*-------------------------------------------------------+
| DataVaccinator Vault Provider System
| Copyright (C) DataVaccinator
| https://www.datavaccinator.com/
+--------------------------------------------------------+
| Filename: perf_test.php
| Author: Data Vaccinator Development Team
+--------------------------------------------------------+
| This program is released as free software under the
| Affero GPL license. You can redistribute it and/or
| modify it under the terms of this license which you
| can read by viewing the included agpl.txt or online
| at www.gnu.org/licenses/agpl.html. Removal of this
| copyright header is strictly prohibited without
| written permission from the original author(s).
+--------------------------------------------------------*/

if (count($argv) < 4) {
    print("Please provide ProviderID Password URL{:Port} durationSec clients\n");
    exit();
}

require_once(__DIR__ . '/util.php'); // include common functions 

$serviceProviderID  = $argv[1];
$serviceProviderPwd = $argv[2];
$url = $argv[3] . "/index.php";
$duration = getFromHash($argv, 4, 0);
$clients = getFromHash($argv, 5, 0);

if ($duration < 1) { $duration = 10; } // 10 sec default
if ($clients < 1) { $clients = 4; } // 4 clients default

$call = "php perf_client.php $serviceProviderID $serviceProviderPwd $url $duration";

$descriptorspec = array(
    0 => array("pipe", "r"),   // stdin is a pipe that the child will read from
    1 => array("pipe", "w"),   // stdout is a pipe that the child will write to
    2 => array("pipe", "w")    // stderr is a pipe that the child will write to
);

$proc = array();

for ($client=0; $client < $clients; $client++) { 
    print("Start client $client...\n");
    exec("$call > output$client.txt &");
}

sleep($duration + 3); // wait 3 extra seconds

for ($client=0; $client < $clients; $client++) { 
    $t = file_get_contents("output$client.txt");
    print($t."\n");
    unlink("output$client.txt");
}

print("Done\n");

exit(0);
