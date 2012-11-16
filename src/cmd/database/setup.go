// Copyright 2012 The Ephenation Authors
//
// This file is part of Ephenation.
//
// Ephenation is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3.
//
// Ephenation is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with Ephenation.  If not, see <http://www.gnu.org/licenses/>.
//

package main

import (
	"ephenationdb"
	"flag"
	"github.com/robfig/goconfig/config"
	"log"
	"os"
)

var (
	configFileName = flag.String("configfile", "config.ini", "General configuration file")
	logOnStdout    = flag.Bool("s", false, "Send log file to standard otput")
	logFileName    = flag.String("log", "database.log", "Log file name")
)

func main() {
	if !*logOnStdout {
		logFile, _ := os.OpenFile(*logFileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
		log.SetOutput(logFile)
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	cnfg, err := config.ReadDefault(*configFileName)
	if err != nil {
		log.Println("Fail to find", *configFileName, err)
		return
	}
	if cnfg.HasSection("sql") {
		server, err := cnfg.String("sql", "DatabaseServer")
		if err != nil {
			log.Println(*configFileName, "DatabaseServer:", err)
			return
		}
		name, err := cnfg.String("sql", "DatabaseName")
		if err != nil {
			log.Println(*configFileName, "DatabaseName:", err)
			return
		}
		login, err := cnfg.String("sql", "DatabaseLogin")
		if err != nil {
			log.Println(*configFileName, "DatabaseLogin:", err)
			return
		}
		pwd, err := cnfg.String("sql", "DatabasePassword")
		if err != nil {
			log.Println(*configFileName, "DatabasePassword:", err)
			return
		}
		err = ephenationdb.SetConnection(server, name, login, pwd)
		if err != nil {
			log.Println(*configFileName, "DatabasePassword:", err)
			return
		}
	} else {
		log.Println("Config file", configFileName, "missing, or no section 'sql'")
	}
	db := ephenationdb.New()
	log.Println(db)
}
