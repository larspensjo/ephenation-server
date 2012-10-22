#!/bin/sh
cp ../dumpfile.sql .
strip server
strip shell
tar cvfz distro-linux64-`date +%F`.gz server shell dumpfile.sql readme.md config.ini
rm dumpfile.sql
