/*
	Binance Intelligence Terminal in Go
    Copyright (C) <2021> <infl00p Labs>

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

package main

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

// FillSymbolStats ...
// This function gets called periodically to find all relevant symbol pairs
// and update them
func FillSymbolStats(stats map[string]Ticker, db *sql.DB) {
	// Get distinct pairs
	query := "select distinct(symbol) from " +
		"(select symbol,timestamp from events union " +
		"select symbol,timestamp from trades) " +
		"where datetime(timestamp) >= datetime('now','-" +
		conf.Db.SamplePeriod + "')"
	rows, err := db.Query(query)
	if err == sql.ErrNoRows {
		return
	} else if err != nil {
		log.Println("Error executing FillSymbolStats query " + err.Error())
		return
	}

	for rows.Next() {
		var pair string
		err = rows.Scan(&pair)
		if err != nil {
			log.Printf("Error Fetching pairs \n")
		}
		stats[pair] = GetSymbolTicker(pair)
	}
}

// GetSymbolTicker returns Ticker structure
// Rest API call to get ticker information about a specific
// symbol pair
func GetSymbolTicker(s string) Ticker {
	var sym Ticker
	client := &http.Client{}
	req, err := http.NewRequest("GET", restapiurl+"ticker/24hr?symbol="+s, nil)
	if err != nil {
		log.Println(err.Error())
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err.Error())
	}

	// parse json response
	body, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal([]byte(body), &sym)

	return sym
}

// GetSymbolInfo returns error
// Rest API call to get exchange information for specific
// symbol pair
func GetSymbolInfo (s string, m map[string]Symbol) error {
	type ExchangeInfo struct {
		Symbols	[]Symbol `json:"symbols"`
	}

	sym := strings.ToUpper(s)
	log.Println("Getting Info for ",sym , s)
	var info ExchangeInfo
	client := &http.Client{}
	req, err := http.NewRequest("GET", restapiurl+"exchangeInfo?symbol="+sym, nil)
	if err != nil {
		log.Println(err.Error())
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err.Error())
	}

	// parse json response
	body, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal([]byte(body), &info)

	m[s] = info.Symbols[0]

	return err
}

// SubscribeToTrades ...
// Pushes aggregated trade subscribe string to generic websocket channel
func SubscribeToTrades( tx chan<- string, symbol string, quota string) {
	s := strings.ToLower(strings.Split(symbol, "/")[0]+quota)+"@aggTrade"
	tx <- s
}

// ParseEvent returns event structure and error
// Unmarshals the abnormal events JSON string
func ParseEvent(msg string) (ev Event, err error) {
	err = json.Unmarshal([]byte(msg), &ev)
	return
}

// FilterEvent ...
// Unused for now
func FilterEvent(ev Event, quotafilter, basefilter string, percentfilter float32) bool {
	if quotafilter == "" && basefilter == "" && percentfilter == 0 {
		return true
	}
	q, b, p := true, true, true
	if quotafilter != "" {
		q = false
		for _, v := range strings.Split(quotafilter, ",") {
			if v == ev.Data.QuotaAsset {
				q = true
				break
			}
		}
	}
	if basefilter != "" {
		b = false
		for _, v := range strings.Split(quotafilter, ",") {
			if v == ev.Data.BaseAsset {
				b = true
				break
			}
		}
	}
	if percentfilter > 0 {
		if percentfilter < ev.Data.PriceChange*100 {
			p = false
		}
	} else if percentfilter < 0 {
		if percentfilter > ev.Data.PriceChange*100 {
			p = false
		}
	}
	if q && b && p {
		return true
	}
	return false
}

// FilterTrade returns boolean
// Filter Trade streams based on a price threshhold
func FilterTrade( tr Trade, info map[string]Symbol, stats map[string]Ticker, tradestats *TradeStat ) bool {
	// Price Threshhold
	threshhold := conf.Trades.Threshhold
	var pricelimit float64
	if info[tr.Data.Symbol].Symbol == "" || stats[tr.Data.Symbol].Name == "" {
		stats[tr.Data.Symbol] = GetSymbolTicker(tr.Data.Symbol)
		err := GetSymbolInfo(tr.Data.Symbol, info)
		if err != nil {
			return false
		}
	}

	// Convert price limit to default quote asset
	quote := info[tr.Data.Symbol].QuoteAsset
	if quote != conf.Trades.DefaultQuote {
		if stats[quote+conf.Trades.DefaultQuote].LastPrice == 0 {
			stats[quote+conf.Trades.DefaultQuote] = GetSymbolTicker(quote+conf.Trades.DefaultQuote)
		}
		if stats[quote+conf.Trades.DefaultQuote].LastPrice > 0 {
			pricelimit = threshhold / stats[quote+conf.Trades.DefaultQuote].LastPrice
		} else {
			pricelimit = threshhold
		}

	} else {
		pricelimit = threshhold
	}

	// Update Trade Stats
	if tradestats.Number % 1000 == 0 {
		tradestats.Maker=tradestats.Maker/1000
		tradestats.Taker=tradestats.Taker/1000
	}

	if tr.Data.IsMaker {
		tradestats.Maker+=tr.Data.Quantity*tr.Data.Price*pricelimit/threshhold
	} else {
		tradestats.Taker+=tr.Data.Quantity*tr.Data.Price*pricelimit/threshhold
	}
	tradestats.Number++

	// Check if trade is over the quota amount limit
	if tr.Data.Price*tr.Data.Quantity >= pricelimit {
		if conf.DisableLogging == false {
			log.Println(pricelimit, tr)
		}
		return true
	}
	return false
}
