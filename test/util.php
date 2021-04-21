<?php
/*-------------------------------------------------------+
| DataVaccinator Vault Provider System
| Copyright (C) DataVaccinator
| https://www.datavaccinator.com/
+--------------------------------------------------------+
| Filename: util.php
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

/*
 * General functions used by the DataVaccinator test script.
 * 
 * NOTE: Needs PHP curl support/module.
 *       https://www.php.net/manual/en/book.curl.php
 *       Try installation using 
 *       * Ubuntu: apt-get install php-curl
 *       * Suse: zypper install php7-curl
 *       * RedHat: yum install php-curl
 */

/**
 * Returns $key from $hash or $default if either don't exist. Does so without
 * triggering an E_NOTICE event.
 *
 * @param array  $hash    array to find key in
 * @param string $key   key to look up
 * @param string $default default value to return when key is not found
 * @return mixed value or default
 */
function getFromHash($hash, $key, $default = "") {
    return isset($hash[$key])?$hash[$key]:$default;
}

/**
* Post the given data array to url and returns the response without the headers
* or FALSE in case of an error.
*
* Do not mix a data value string (body) with files array!
*
* @param string $url the url to post the data to
* @param mixed $data the data to post (array for form fields or string for body)
* @param string &$error where the error is written to in case of failure
* @param int $TimeoutSec function timeout in seconds
* @return mixed the body of the response without the headers.
*/
function DoRequest($url, $data, &$error = "", $TimeoutSec = 8) {
    $h = curl_init();
    $error = '';
    If ($h == 0) {
        $error .= "Error calling curl_init. Check your cURL setup.";
        return False;
    }
    $isSSL = strtolower(substr($url, 0 ,5)) == "https";
    $ret = false;

    do {
        // setup SSL here
        if ($isSSL) {
            # force to not verify SSL hosts!
            If (! curl_setopt($h, CURLOPT_SSL_VERIFYPEER, false)) {
                $error .= "Error setting CURLOPT_SSL_VERIFYPEER. Curl ec:".
                            curl_error($h);
                break;
            }
            # 2=validate hostname of peer-certificate
            If (! curl_setopt($h, CURLOPT_SSL_VERIFYHOST, 2)) {
                $error .= "Error setting CURLOPT_SSL_VERIFYHOST. Curl ec:".
                            curl_error($h);
                break;
            }
        }
        
        # dont output header in result
        If (! curl_setopt($h, CURLOPT_HEADER, false)) {
            $error .= "Error setting CURLOPT_HEADER. Curl ec:". curl_error($h);
            break;
        }

        # set connection url
        If (! curl_setopt($h, CURLOPT_URL, $url)) {
            $error .= "Error setting URL. Curl ec:" . curl_error($h);
            break;
        }

        if (is_array($data) == TRUE && count($data) > 0) {
            // set post data only, if array contains values
            if (! curl_setopt($h, CURLOPT_POSTFIELDS, $data)) {
                $error .= "Error setting POSTFIELDS. Curl ec:" . curl_error($h);
                break;
            }
        } else {
            if (is_string($data) == true && strlen($data) > 0) {
                if (! curl_setopt($h, CURLOPT_POSTFIELDS, $data)) {
                    $error .= "Error setting POSTFIELDS. Curl ec:" . curl_error($h);
                    break;
                }
            }
        }

        # return result with exec
        If (! curl_setopt($h, CURLOPT_RETURNTRANSFER, true)) {
            $error .= "Error setting CURLOPT_RETURNTRANSFER. Curl ec:". curl_error($h);
            break;
        }

        # set timeout for this function
        If (! curl_setopt($h, CURLOPT_CONNECTTIMEOUT, $TimeoutSec)) {
            $error .= "Error setting CURLOPT_RETURNTRANSFER. Curl ec:". curl_error($h);
            break;
        }
        
        # only lookup IPv4 address here
        If (! curl_setopt($h, CURLOPT_IPRESOLVE, CURL_IPRESOLVE_V4)) {
            $error .= "Error setting CURLOPT_IPRESOLVE. Curl ec:". curl_error($h);
            break;
        }

        $ret = @curl_exec($h);
        $code = curl_getinfo($h, CURLINFO_HTTP_CODE);
        if ($ret === FALSE) {
            $error .= "Error executing curl call ($code). Error:".curl_error($h);
        }

    } while (false);

    curl_close($h);
    return $ret;
}
?>