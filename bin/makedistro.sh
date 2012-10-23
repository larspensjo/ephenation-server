#!/bin/sh
cp ../dumpfile.sql .
strip server shell clientsimulator
tar cvfz distro-linux64-`date +%F`.gz server shell clientsimulator dumpfile.sql readme.md config.ini
rm dumpfile.sql
