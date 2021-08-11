#!/bin/sh

: '
+--------------------------------------------------------+
| DataVaccinator Vault Provider System
| Copyright (C) DataVaccinator
| https://www.datavaccinator.com/
+--------------------------------------------------------+
| Filename: install.sh
| Author: Data Vaccinator Development Team
+--------------------------------------------------------+
| This program is released as free software under the
| Affero GPL license. You can redistribute it and/or
| modify it under the terms of this license which you
| can read by viewing the included agpl.txt or online
| at www.gnu.org/licenses/agpl.html. Removal of this
| copyright header is strictly prohibited without
| written permission from the original author(s).
+--------------------------------------------------------+
'

# This script controls the pre-configuration for the
# vaccinator executable, it's autostart with systemd and
# the initial creation of the database, db-user and tables.

echo " __                                 "
echo "|  \\ _ |_ _ \\  /_  _ _. _  _ |_ _  _ "
echo "|__/(_|| (_| \\/(_|(_(_|| )(_|| (_)|  "
echo "                            Installer"
echo " "

if ! [ $(id -u) = 0 ]; then
   echo "Please run me as root or use sudo!"
   exit 1
fi

if ! [ -x "$(command -v cockroach)" ]; then
    echo "'cockroach' commandline tool could not be found. Make sure CockroachDB is installed!"
    exit 1
fi

if ! [ -x "$(command -v dig)" ]; then
    echo "'dig' commandline tool could not be found. Please install dig first (install bind-utils)!"
    exit 1
fi

if ! [ -x "$(command -v curl)" ]; then
    echo "'curl' commandline tool could not be found. Please install curl first!"
    exit 1
fi

DIST=$( cat /etc/*-release | tr [:upper:] [:lower:] | grep -Poi '(debian|ubuntu|red hat|centos|suse|arch|rocky)' | head -n 1 )
if [ -z "$DIST" ]
then
    echo "This distribution is not supported by this script. Sorry."
    exit 1
fi

echo -n "Where shall I install DataVaccinator Vault executable (/opt/vaccinator): "
read dvpath
if [ -z "$dvpath" ]
then
    dvpath="/opt/vaccinator"
fi
echo $dvpath

default=$( curl -s ifconfig.me )
echo -n "What IP shall the DataVaccinator Vault listen to ($default): "
read dvip
if [ -z "$dvip" ]
then
    dvip=$default
fi
echo $dvip

echo -n "What port shall DataVaccinator Vault listen to (443): "
read dvport
if [ -z "$dvport" ]
then
    dvport="443"
fi
echo $dvport

useSSL="0"
testdomain=$(dig -x $dvip +short)
testdomain=${testdomain%?}
domain=""
if [ $dvport = "443" ]
then
    useSSL="1"
    echo -n "What is the domain name ($testdomain): "
    read domain
    if [ -z "$domain" ]
    then
        domain=$testdomain
    fi
    echo $domain
fi

echo -n "What IP is the database listening to (127.0.0.1): "
read dbip
if [ -z "$dbip" ]
then
    dbip="127.0.0.1"
fi
echo $dbip

echo -n "What is the database user name to use (dv): "
read dvuser
if [ -z "$dvuser" ]
then
    dvuser="dv"
fi
echo $dvuser

echo "--------------------------- database creation"
echo -n "Prepare SQL database update script... "
if sed -e"s|<USER>|$dvuser|g" "./database.sql" > "./database.tmp"
then
    echo "OK"
    echo "Execute SQL database update script... "
    if cockroach sql --host $dbip --insecure < "./database.tmp"
    then
        echo "SQL script OK"
    else
        echo "SQL script import FAILED (maybe DB already there)"
    fi
    rm "./database.tmp"
else
    echo "FAILED TO GENERATE database.tmp"
fi

echo "--------------------------- user creation"

echo "Create a system user and group 'vaccinator' for running the vaccinator... ($DIST) "

if [ "$DIST" = "ubuntu" -o "$DIST" = "debian" ]
then
    if ! adduser --system --no-create-home --group vaccinator
    then
        echo "FAILED user creation"
    fi
elif [ "$DIST" = "centos" -o "$DIST" = "red hat" -o "$DIST" = "rocky" ]
then
    if ! adduser --system --no-create-home vaccinator
    then
        echo "FAILED user creation"
    fi
elif [ "$DIST" = "suse" -o "$DIST" = "arch" ]
then
    if ! useradd -r -U -s /usr/bin/nologin vaccinator
    then
        echo "FAILED user creation"
    fi
fi

echo " "
echo "--------------------------- copy files"

echo -n "Create folder $dvpath... "
if mkdir -p $dvpath
then
    echo "OK"
    chown vaccinator:vaccinator $dvpath
else
    echo "FAILED"
fi

echo "Try to stop any running vaccinator deamon... (may fail if not running)"
systemctl stop vaccinator.service

echo -n "Copy vaccinator executable to $dvpath... "
# -> owner 'vaccinator', group 'vaccinator', make executable
if install -o vaccinator -g vaccinator -m +x "./vaccinator" "$dvpath/"
then
    echo "OK"
else
    echo "FAILED"
    echo "Stop because the installation source is missing the 'vaccinator' executable!"
    exit 1
fi

echo -n "Copy vaccinator configuration file (config.json) to $dvpath... "

if [ ! -f "$dvpath/config.json" ]; 
then

    configString="user=$dvuser host=$dbip port=26257 dbname=vaccinator"
    if sed -e"s|<PORT>|$dvport|g" \
           -e"s|<CONN>|$configString|g" \
           -e"s|<CERTS>|$dvpath/certs/|g" \
           -e"s|<USER>|vaccinator|g" \
           -e"s|<USESSL>|$useSSL|g" \
           -e"s|<DOMAIN>|$domain|g" \
           -e"s|<IP>|$dvip|g" \
           "./config.json" > "$dvpath/config.json"
    then
        echo "OK"
        chown vaccinator:vaccinator "$dvpath/config.json"
    else
        echo "FAILED"
    fi
else
    echo "ALREADY THERE"
fi

echo "--------------------------- systemd integration"

echo -n "Do you want me to create/update a systemd autostart entry for vaccinator (Y/n): "
read wantAutostart
if [ -z "$wantAutostart" -o "$wantAutostart" = "y" -o "$wantAutostart" = "Y" ]
then
    echo -n "Create a systemd file in /etc/systemd/system folder... "
    if sed -e"s|<PATH>|$dvpath|g" -e"s|<USER>|vaccinator|g" "./vaccinator.service" > "/etc/systemd/system/vaccinator.service"
    then
        echo "OK"
        echo -n "Activate vaccinator.service... "
        if systemctl enable vaccinator.service
        then
            echo "OK"
            systemctl daemon-reexec
        else
            echo "FAILED"
        fi
    else
        echo "FAILED"
    fi
    echo "---------------------------"
fi
echo " "
echo "Hints:"
echo "- Configure DataVaccinator in '$dvpath/config.json'"
echo "- Run daemon using 'systemctl start vaccinator'"
echo "- Validate service logs using 'journalctl -et vaccinator'"
echo " "
echo "Finished"