/*
	Binance Intelligence Terminal in Go
    Copyright (C) <2021-2023> <infl00p Labs>

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.

*/

package config

import (
	"log"
	"os"
	"time"

	"github.com/jinzhu/configor"
	"github.com/shibukawa/configdir"
)

// Configuration with default values
// Equivalent Json config file:
//
//	{
//		"BinanceTerminal": "https://www.binance.com/en/trade/",
//		"EnableMouse":	"true",
//		"DisableTimer":	"30s",
//		"DisableLogging":	"false",
//		"Db" : {
//			"Retention": "1 hours",
//			"SamplePeriod": "10 minutes"
//			"InMemory":	 "true",
//		}
//		"Trades" : {
//			"Quotes": ["USDT", "BTC", "BNB", "ETH"],
//			"DefaultQuote": "USDT",
//			"Threshhold": 50000
//		}
var Conf = struct {
	BinanceTerminal string        `default:"https://www.binance.com/en/trade/"`
	EnableMouse     bool          `default:"true"`
	TickerTimer     time.Duration `default:"30s"`
	DisableLogging  bool          `default:"false"`
	Db              struct {      // SQL Syntax
		Retention    string `default:"1 hours"`
		SamplePeriod string `default:"10 minutes"`
		InMemory     bool   `default:"true"`
	}
	Trades struct {
		Quotes       []string `default:"[USDT, BTC, BNB, ETH]"`
		DefaultQuote string   `default:"USDT"`
		Threshhold   float64  `default:"50000"`
	}
}{}

var Storagepath string

func init() {
	// Configuration Init
	configdirs := configdir.New(Vendorname, Appname)
	localcache := configdirs.QueryCacheFolder()
	localconfig := configdirs.QueryFolderContainsFile("config.json")
	localcache.MkdirAll()

	// Load Config and configuration file
	if localconfig != nil {
		err := configor.Load(&Conf, localconfig.Path+"/config.json")
		if err != nil {
			log.Fatal(err.Error())
			os.Exit(1)
		}
	} else {
		err := configor.Load(&Conf, "config.json")
		if err != nil {
			log.Fatal(err.Error())
			os.Exit(1)
		}
	}

	Storagepath = localcache.Path

}
