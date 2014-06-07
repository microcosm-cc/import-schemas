Description
===========

Imports files exported according to [export-schemas](https://github.com/microcosm-cc/export-schemas) into Microcosm.

Usage
=====

Building
--------

`make`

This will also install the built binary into your `$(GOPATH)/bin` directory.

Configuring
-----------

Modify `config.toml` to reflect the exports directory, site attributes and database credentials.

````
[database]
host=sql.dev.microcosm.cc
port=5432
database=microcosm
username=microcosm
password=changeme

[site]
name = LFGSS
description = London Fixed-Gear and Single-Speed
subdomain_key = lfgssimport
owner_id = 1

[export]
rootpath = ./exported
````

If the subdomain_key matches any existing site, the import will put the data into that site.

Running
-------

Assuming `$(GOPATH)/bin` is in your `$(PATH)` then:

`import-schemas`

Progress will be printed to stdout.

By default logging (not verbose) will go to the /tmp/ directory. You can change this and include verbose logging by running with flags:

`import-schemas -log_dir=. -v=2`

That will configure verbose logging and will log to the local directory. Flags available can be seen in [glog](https://github.com/golang/glog).

Design Principles
=================

1. The import must be resumable
2. The import must handle all dependencies
3. The import must leave the data in an integral state

Resumable
---------

Walk the exported directory to determine the work to be done, extract identifiers and only do the work if we can prove the identifier has not already been processed.

The importItem() functions are idempotent. If a profile has already been imported and `importProfile` is called again with the same parameters, this is not considered an error and should return the already imported profileId. This means the import tool can be run multiple times on the same (or slightly differing) dataset.

Dependencies
------------

Users must be imported before forums can be created, comments must be imported before attachments can be created against them, etc.

Generally, so long as we process all users before forums, all forums before conversations, all conversations before comments, etc... we'll be fine.

We can import non-dependent items concurrently, but for comments we must do them serially as some comments reference others (inReplyTo).

Integrity
---------

Microcosm expects a certain state of the data that is in part achieved through integrity management within Microcosm. If we're not using Microcosm then we are responsible for making sure that we leave the data as Microcosm expects it and that Microcosm works against the imported data.