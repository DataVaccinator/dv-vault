<?php
/*-------------------------------------------------------+
| DataVaccinator Vault Provider System
| Copyright (C) DataVaccinator
| https://www.datavaccinator.com/
+--------------------------------------------------------+
| Filename: perf_client.php
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

if (count($argv) < 5) {
    print("Please provide ProviderID Password URL{:Port} durationSec\n");
    print("For single instance test only. Consider running perf_test.php instead.\n");
    exit();
}

require_once(__DIR__ . '/util.php'); // include common functions 

$serviceProviderID  = $argv[1];
$serviceProviderPwd = $argv[2];
$url = $argv[3];
$duration = $argv[4];

_doCheck(); // check connectivity

$startTime = time();
$count = 0;

do {
    // main loop
    $needed = time() - $startTime;
    if ($needed > $duration) { break; }

    $vid = _doCreate();
    _doGet($vid);
    _doDelete($vid);

    $count = $count + 1;

} while(true);

$needed = time() - $startTime;
print("Duration: $needed sec\n");
print("Transactions: $count (each create, retrieve and delete)\n");
print("Operations/sec: ".$count / $needed * 3 . "\n");

exit(0);

function _doCheck() {
    global $url,$serviceProviderID, $serviceProviderPwd, $someKey;
    $r = array();
    $r["version"] = 2;
    $r["sid"] = $serviceProviderID;
    $r["spwd"] = $serviceProviderPwd;
    $r["op"] = "check";
    $j = _parseVaccinatorResult(json_encode($r));
    if ($j === NULL || $j === false) { print("Unexpected result (no json)\n"); exit(1); }
    if ($j["status"] != "OK") {
        print "Expected status OK for op 'check', got [".$j["status"]."] instead.\n";
        exit(1);
    }
    print("Connection to $url is working.\n");
}

function _doCreate() {
    global $serviceProviderID, $serviceProviderPwd, $someKey;
    $r = array();
    $r["version"] = 2;
    $r["sid"] = $serviceProviderID;
    $r["spwd"] = $serviceProviderPwd;
    $r["op"] = "add";
    $r["data"] = "chacha20:7f:29a1c8b68d8a:btewwyzox3i3fe4cg6a1qzi8pqoqa55orzf4bcxtjfcf5chep998sj6";
    $r["uid"] = 12345;
    $j = _parseVaccinatorResult(json_encode($r));
    return getFromHash($j, "vid");
}

function _doGet($vid) {
    global $serviceProviderID, $serviceProviderPwd, $someKey;
    $r = array();
    $r["version"] = 2;
    $r["sid"] = $serviceProviderID;
    $r["spwd"] = $serviceProviderPwd;
    $r["op"] = "get";
    $r["uid"] = 12345;
    $r["vid"] = $vid;
    $j = _parseVaccinatorResult(json_encode($r));
    if ($j["status"] != "OK") {
        print "Expected status OK for 'get' operation, got [".$j["status"]."] instead.\n";
        return false;
    }
    return true;
}

function _doDelete($vid) {
    global $serviceProviderID, $serviceProviderPwd, $someKey;
    $r = array();
    $r["version"] = 2;
    $r["sid"] = $serviceProviderID;
    $r["spwd"] = $serviceProviderPwd;
    $r["op"] = "delete";
    $r["uid"] = 12345;
    $r["vid"] = $vid;
    $j = _parseVaccinatorResult(json_encode($r));
    if ($j["status"] != "OK") {
        print "Expected status OK for 'delete' operation, got [".$j["status"]."] instead.\n";
        return false;
    }
    return true;
}

/**
 * *******************************************
 * HELPING FUNCTIONS BELOW
 * *******************************************
 */

/**
 * Call DataVaccinator Vault and decode result.
 * 
 * @param string $json
 * @return array
 */
function _parseVaccinatorResult($json) {
    global $url;
    $data = array();
    $data["json"] = $json;
    $error = "";
    $res =  DoRequest($url, $data, $error, 8);
    $j = json_decode($res, true);
    return $j;
}