<?php
/*-------------------------------------------------------+
| DataVaccinator Vault Provider System
| Copyright (C) DataVaccinator
| https://www.datavaccinator.com/
+--------------------------------------------------------+
| Filename: migrate.php
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

// This script migrates all MySQL data from a previous
// DataVaccinator system to the new CockroachDB.

// ----------- CONFIGURATION -----------
$mySqlUser = "root";
$mySqlPass = "pwd";
$myConnectionString = "host=localhost;dbname=vaccinator";

$cdbUser = "dv";
$cdbPass = "pwd";
$cdbConnectionString = "user='$cdbUser' password='$cdbPass' host=localhost port=26257 dbname=vaccinator";

// --------- END CONFIGURATION ---------

print("Connect CockroachDB... ");
try {
    $cdb = new PDO("pgsql:$cdbConnectionString",
        $cdbUsername, null, array(
        PDO::ATTR_ERRMODE          => PDO::ERRMODE_EXCEPTION,
        PDO::ATTR_EMULATE_PREPARES => true,
        PDO::ATTR_PERSISTENT => true
        ));

} catch (Exception $e) {
    print("FAILED\nUnable to connect to CockroachDB: ".$e->getMessage()." (".$e->getCode().")\n");
    exit;
}
print("OK\n");

print("Connect MySQL Database... ");
try {
    $mdb = new PDO("mysql:$myConnectionString", $mySqlUser, $mySqlPass);

} catch (Exception $e) {
    print("FAILED\nUnable to connect to MySQL: ".$e->getMessage()." (".$e->getCode().")\n");
    exit;
}
print("OK\n");

// -------------- PROVIDER --------------

print("\nMigrate provider entries... \n");

// prepare CockroachDB for insert
$cdb->beginTransaction();

// select and insert in 100 bulk
$cnt = 0;
$result = $mdb->query("SELECT * FROM provider");
print("- Run provider migration...\n");
while ($data = $result->fetch(PDO::FETCH_ASSOC)) {
    try {
        $SQL = "INSERT INTO provider (providerid, name, password, ip, creationdate) 
                VALUES (?,?,?,?,?)";
        $prep = $cdb->prepare($SQL);
        if ($data["CREATIONDATE"] == 0) {
            $data["CREATIONDATE"] = "2000-01-01 00:00:00";
        }
        $cret = $prep->execute(array($data["PROVIDERID"], 
                                     $data["NAME"], 
                                     $data["PASSWORD"], 
                                     $data["IP"], 
                                     $data["CREATIONDATE"]));
        if ($cret) {
            $cnt = $cnt + 1;
        }
    } catch (Exception $e) {
        if ($e->getCode() != 23505) {
            print("ERROR: ".$e->getMessage()."\n");
        }
    }
}
print("- Done for $cnt entries\n");

// commit CockroachDB inserts
$cdb->commit();

print("OK\n");

// -------------- DATA --------------

print("\nMigrate data entries...\n");

print("- Create migration column... ");
$r = $mdb->exec("ALTER TABLE data ADD COLUMN mig SMALLINT DEFAULT 0");
if ($r === false) {
    print("COLUMN 'mig' ALREADY EXISTS\n");
} else {
    print("OK\n");
}

do {
    // prepare CockroachDB for insert
    $cdb->beginTransaction();

    // select and insert in 100 bulk
    $cnt = 0;
    $result = $mdb->query("SELECT * FROM data WHERE mig=0 LIMIT 100");
    print("- Chunk with 100 entries...\n");
    while ($data = $result->fetch(PDO::FETCH_ASSOC)) {
        try {
            $SQL = "INSERT INTO data (vid, payload, providerid, creationdate) 
                    VALUES (?,?,?,?)";
            $prep = $cdb->prepare($SQL);
            $cret = $prep->execute(array($data["PID"], 
                                        $data["PAYLOAD"], 
                                        $data["PROVIDERID"], 
                                        $data["CREATIONDATE"]));
            if ($cret) {
                $mdb->exec("UPDATE data SET mig=1 
                            WHERE PID='".$data["PID"]."'");
                $cnt = $cnt + 1;
            }
        } catch (Exception $e) {
            print("ERROR: ".$e->getMessage()."\n");
        }
    }
    print("- Done for $cnt entries\n");

    // commit CockroachDB inserts
    $cdb->commit();

} while ($cnt > 99);

print("OK\n");

// -------------- WORDS --------------

print("\nMigrate word entries...\n");

print("- Create migration column... ");
$r = $mdb->exec("ALTER TABLE search ADD COLUMN mig SMALLINT DEFAULT 0");
if ($r === false) {
    print("COLUMN 'mig' ALREADY EXISTS\n");
} else {
    print("OK\n");
}

do {
    // prepare CockroachDB for insert
    $cdb->beginTransaction();

    // select and insert in 100 bulk
    $cnt = 0;
    $result = $mdb->query("SELECT * FROM search WHERE mig=0 LIMIT 100");
    print("- Chunk with 100 entries...\n");
    while ($data = $result->fetch(PDO::FETCH_ASSOC)) {
        try {
            $SQL = "INSERT INTO search (vid, word) 
                    VALUES (?,?)";
            $prep = $cdb->prepare($SQL);
            $cret = $prep->execute(array($data["PID"], 
                                        $data["WORD"]
                                        ));
            if ($cret) {
                $mdb->exec("UPDATE search SET mig=1 
                            WHERE PID='".$data["PID"]."' AND 
                                 WORD='".$data["WORD"]."'");
                $cnt = $cnt + 1;
            }
        } catch (Exception $e) {
            print("ERROR: ".$e->getMessage()."\n");
        }
    }
    print("- Done for $cnt entries\n");

    // commit CockroachDB inserts
    $cdb->commit();

} while ($cnt > 99);

print("OK\n");

// -------------- LOG --------------

print("\nMigrate log/audit entries...\n");

print("- Create migration column... ");
$r = $mdb->exec("ALTER TABLE log ADD COLUMN mig SMALLINT DEFAULT 0");
if ($r === false) {
    print("COLUMN 'mig' ALREADY EXISTS\n");
} else {
    print("OK\n");
}

do {
    // prepare CockroachDB for insert
    $cdb->beginTransaction();

    // select and insert in 100 bulk
    $cnt = 0;
    $result = $mdb->query("SELECT * FROM log WHERE mig=0 LIMIT 100");
    print("- Chunk with 100 entries...\n");
    while ($data = $result->fetch(PDO::FETCH_ASSOC)) {
        try {
            $SQL = "INSERT INTO audit (logtype, logdate, providerid, logcomment) 
                    VALUES (?,?,?,?)";
            $prep = $cdb->prepare($SQL);
            $cret = $prep->execute(array($data["LOGTYPE"], 
                                        $data["LOGDATE"],
                                        $data["PROVIDERID"],
                                        $data["LOGCOMMENT"]
                                        ));
            if ($cret) {
                $mdb->exec("UPDATE log SET mig=1 
                            WHERE LOGID=".$data["LOGID"]."");
                $cnt = $cnt + 1;
            }
        } catch (Exception $e) {
            print("ERROR: ".$e->getMessage()."\n");
        }
    }
    print("- Done for $cnt entries\n");

    // commit CockroachDB inserts
    $cdb->commit();

} while ($cnt > 99);

print("OK\n");

// -------------- VALIDATION --------------

print("\nValidate migration...\n");
$tables = array("provider"=>"provider", 
                "data"=>"data",
                "search"=>"search",
                "log"=>"audit"
                );

foreach ($tables as $source => $dest) {
    print("Compare $source ➜ $dest... ");
    $result = $mdb->query("SELECT COUNT(*) AS CNT FROM $source");
    $mret = $result->fetchAll();
    $mcnt = $mret[0]["CNT"];

    $result = $cdb->query("SELECT COUNT(*) AS cnt FROM $dest");
    $cret = $result->fetchAll();
    $ccnt = $cret[0]["cnt"];

    print($mcnt . "➜" . $ccnt . " which is ");
    if ($mcnt == $ccnt) {
        print("OK\n");
    } else {
        print("WRONG!!!\n");
    }
}

print("\nDone, check above validation results.\n\n");
?>