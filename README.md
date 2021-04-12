# DataVaccinator Vault
This is the DataVaccinator Vault platform. It has to be installed on some Linux environment in order to provide the service.

The DataVaccinator Vault platform offers

* Multi tenant use (multiple service providers served on the same platform).
* Word based search using searchable symmetric encryption techniques (SSE).
* Can use [Let's Encrypt certificates](https://letsencrypt.org/) out of the box.
* Fully automatic IP whitelisting by only allowing API requests from dedicated IP addresses.
* Installer-Script supports **CentOS**, **Red Hat**, **Arch Linux**, **Suse Linux**, **Ubuntu** and **Debian** systems (allx86_64).
* Installer-Script automatically generates SystemD daemon, system user and group, database user and database structure.
* DataVaccinator Vault supports working behind proxy servers and loadbalancers (like [HAProxy](http://www.haproxy.org/)).

It requires:

* Some Linux server (Intel/AMD, x86_64)
* [CockroachDB](https://www.cockroachlabs.com/product) database and drivers

What is missing:

* There is currently no web-gui for managing service providers.
* Currently no support for ARM systems.
* Currently no support for Windows Server.


# What is it good for?
The DataVaccinator Vault protects your sensitive data and information against abuse. At the very moment when data is being generated, the service splits that data and uses advanced pseudonymisation techniques to separate content from identity information. Thus, the DataVaccinator Vault reduces cyber security risks in the health, industry, finance and any other sector and helps service providers, device manufacturers, data generating and data handling parties to manage sensitive data in a secure and GDPR-compliant manner. In contrast to other offerings, DataVaccinator industrialises pseudonymisation, thereby making pseudonymisation replicable and affordable. 

Get more information, support and contact at <https://www.datavaccinator.com>.

# License information
DataVaccinator Vault is released as free software under the Affero GPL license (AGPL). You can redistribute it and/or modify it under the terms of this license which you can read by viewing the included agpl.txt or online at www.gnu.org/licenses/agpl.html