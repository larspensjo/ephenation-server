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
	"github.com/larspensjo/config"
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
	configSection := "db"
	if cnfg.HasSection(configSection) {
		f := func(key string) string {
			value, err := cnfg.String(configSection, key)
			if err != nil {
				log.Println("Config file", *configFileName, "Failt to find key", key, err)
				return ""
			}
			return value
		}
		err = ephenationdb.SetConnection(f)
		if err != nil {
			log.Println("main: open DB:", err)
			return
		}
	} else {
		log.Println("Config file", configFileName, "missing, or no section 'db'")
	}
	db := ephenationdb.New()
	chunkdata(db.C("chunkdata"))
}
