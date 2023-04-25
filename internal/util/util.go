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

package util

import (
	"database/sql"
	"encoding/json"
	"gobit/internal/binance"
	. "gobit/internal/config"
	"gobit/internal/data"
	"log"
	"strings"

	"github.com/pkg/browser"
)

type Filter struct {
	Quote   string
	Base    string
	Percent float32
}

// FillSymbolStats ...
// This function gets called periodically to find all relevant symbol pairs
// and update them
func FillSymbolStats(stats map[string]binance.Ticker, db *sql.DB) {
	// Get distinct pairs
	query := "select distinct(symbol) from " +
		"(select symbol,timestamp from events union " +
		"select symbol,timestamp from trades) " +
		"where datetime(timestamp) >= datetime('now','-" +
		Conf.Db.SamplePeriod + "')"
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
		stats[pair] = binance.GetSymbolTicker(pair)
	}
}

// SubscribeToTrades ...
// Pushes aggregated trade subscribe string to generic websocket channel
func SubscribeToTrades(tx chan<- binance.SubChannelMsg, symbol string, quota string) {
	var m binance.SubChannelMsg
	m.Method = "Subscribe"
	m.StreamName = strings.Split(symbol, "/")[0] + quota
	tx <- m
}

// UnSubscribeToTrades ...
// Pushes aggregated trade unsubscribe string to generic websocket channel
func UnSubscribeFromTrades(tx chan<- binance.SubChannelMsg, symbol string) {
	var m binance.SubChannelMsg
	m.Method = "Unsubscribe"
	m.StreamName = strings.Replace(symbol, "/", "", -1)
	tx <- m
}

// UnSubscribeToAllTrades ...
// Pushes aggregated trade unsubscribe all to generic websocket channel
func UnSubscribeFromAllTrades(tx chan<- binance.SubChannelMsg) {
	var m binance.SubChannelMsg
	s := "All"
	m.Method = "UnsubscribeAll"
	m.StreamName = s
	tx <- m
}

// ParseEvent returns event structure and error
// Unmarshals the abnormal events JSON string
func ParseEvent(msg string) (ev binance.Event, err error) {
	err = json.Unmarshal([]byte(msg), &ev)
	return
}

// FilterEvent
func FilterEvent(ev binance.Event, f Filter) bool {
	quote := f.Quote
	base := f.Base
	percent := f.Percent
	if quote == "" && base == "" && percent == 0 {
		return true
	}
	q, b, p := true, true, true
	if quote != "" {
		q = false
		for _, v := range strings.Split(quote, ",") {
			if v == ev.Data.QuotaAsset {
				q = true
				break
			}
		}
	}
	if base != "" {
		b = false
		for _, v := range strings.Split(base, ",") {
			if v == ev.Data.BaseAsset {
				b = true
				break
			}
		}
	}
	if percent > 0 {
		if percent < ev.Data.PriceChange*100 {
			p = false
		}
	} else if percent < 0 {
		if percent > ev.Data.PriceChange*100 {
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
func FilterTrade(tr binance.Trade, info map[string]data.Symbol, stats map[string]binance.Ticker, tradestats *data.TradeStat) bool {
	// Price Threshhold
	threshhold := Conf.Trades.Threshhold
	var pricelimit float64
	if info[tr.Data.Symbol].Symbol == "" || stats[tr.Data.Symbol].Name == "" {
		stats[tr.Data.Symbol] = binance.GetSymbolTicker(tr.Data.Symbol)
		err := binance.GetSymbolInfo(tr.Data.Symbol, info)
		if err != nil {
			return false
		}
	}

	// Convert price limit to default quote asset
	quote := info[tr.Data.Symbol].QuoteAsset
	if quote != Conf.Trades.DefaultQuote {
		if stats[quote+Conf.Trades.DefaultQuote].LastPrice == 0 {
			stats[quote+Conf.Trades.DefaultQuote] = binance.GetSymbolTicker(quote + Conf.Trades.DefaultQuote)
		}
		if stats[quote+Conf.Trades.DefaultQuote].LastPrice > 0 {
			pricelimit = threshhold / stats[quote+Conf.Trades.DefaultQuote].LastPrice
		} else {
			pricelimit = threshhold
		}

	} else {
		pricelimit = threshhold
	}

	// Update Trade Stats
	if tradestats.Number%1000 == 0 {
		tradestats.Maker = tradestats.Maker / 1000
		tradestats.Taker = tradestats.Taker / 1000
	}

	if tr.Data.IsMaker {
		tradestats.Maker += tr.Data.Quantity * tr.Data.Price * pricelimit / threshhold
	} else {
		tradestats.Taker += tr.Data.Quantity * tr.Data.Price * pricelimit / threshhold
	}
	tradestats.Number++

	// Check if trade is over the quota amount limit
	if tr.Data.Price*tr.Data.Quantity >= pricelimit {
		if Conf.DisableLogging == false {
			log.Println(pricelimit, tr)
		}
		return true
	}
	return false
}

func ShowWebTrade(asset string) {
	browser.OpenURL(Conf.BinanceTerminal + asset)
}
