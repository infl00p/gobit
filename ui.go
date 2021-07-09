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
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	_ "github.com/mattn/go-sqlite3"

	"github.com/gdamore/tcell/v2"
	"github.com/pkg/browser"
	"github.com/rivo/tview"
)

// InitLiveFeed ui element init
func InitLiveFeed() (*tview.Table) {
	livefeed := tview.NewTable().
		SetBorders(false).
		SetSeparator(tview.Borders.Vertical).
		SetBordersColor(tcell.ColorGray)
	livefeed.SetBorder(true).SetTitle("Live Feed").
		SetTitleAlign(tview.AlignLeft).
		SetBorderAttributes(tcell.AttrDim)
	livefeed.SetFixed(1, 1)
	return livefeed
}

// InitTrendBar ui element init
func InitTrendBar() (*tview.TextView) {
	trendbar := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(false)
	trendbar.SetBorder(true).SetTitle("Trade Trend").
		SetTitleAlign(tview.AlignLeft).
		SetBorderAttributes(tcell.AttrDim)
	trendbar.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		return action, nil
	})
	return trendbar
}

// InitMomentumTable ui element init
func InitMomentumTable() (*tview.TextView) {
	momentumtable := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(false)
	momentumtable.SetBorder(true).SetTitle("Popularity (" + conf.Db.SamplePeriod + ")").
		SetTitleAlign(tview.AlignLeft).
		SetBorderAttributes(tcell.AttrDim)
	momentumtable.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		return action, nil
	})
	return momentumtable
}

// InitDetailsTable ui element init
func InitDetailsTable() (*tview.TextView) {
	detailstable := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(false)
	detailstable.SetBorder(true).SetTitle("Details (" + conf.TickerTimer.String() + ")").
		SetTitleAlign(tview.AlignLeft).
		SetBorderAttributes(tcell.AttrDim)
	detailstable.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		return action, nil
	})
	return detailstable
}

// PrintMomentumTable function to build the asset momentum table
func PrintMomentumTable(tablewidth int, assetstats []AssetStat) string {
	bargraph := ""
	volumeavg := ""
	var maxmomentum float64
	var maxnamelen int

	if assetstats == nil {
		return "No enough data\n"
	}

	for _, asset := range assetstats {
		if maxmomentum < asset.Momentum {
			maxmomentum = asset.Momentum
		}

		if maxnamelen < len(asset.Name) {
			maxnamelen = len(asset.Name)
		}
	}

	// Assemble Bar Graph Text
	var padding, width int
	var bar, label, leftline string

	for _, asset := range assetstats {
		padding = 3 + maxnamelen
		if tablewidth > 42 {
			padding += 8
			// Optional Volume Average
			if asset.AvgVolume >= 10000 {
				volumeavg = fmt.Sprintf(" %8.2E", asset.AvgVolume)
			} else {
				volumeavg = fmt.Sprintf(" %8.2f", asset.AvgVolume)
			}
		}
		// Momentum Bar
		width = int((asset.Momentum / maxmomentum) * float64(tablewidth))
		if width >= padding {
			rightalign := ""
			if width == padding {
				width++
			}
			bar = strings.Repeat("▱", width-padding)

			// Left Line
			leftline = strings.Repeat(" ", maxnamelen-len(asset.Name)) + "│"

			// Labels
			label = asset.Name

			// Assemble Graph Line
			if tablewidth > 42 {
				rightpadding := tablewidth - utf8.RuneCountInString(label+leftline+bar+volumeavg)
				rightalign = strings.Repeat(" ", rightpadding)
			}
			bargraph += label + leftline + bar + rightalign + volumeavg + "\n"
		}
	}
	return strings.TrimSuffix(bargraph, "\n")
}

// UpdateTrendBar - prints the trend bar
func UpdateTrendBar(width int, tradestats *TradeStat) string {
	var redboxes, greenboxes string

	// Calculate cells based on rounded percentage
	if tradestats.Maker == 0 || tradestats.Taker == 0 || tradestats.Number == 0 {
	} else {
		trendpercent := tradestats.Maker / (tradestats.Taker+tradestats.Maker)
		if width > 3 {
			leftpadding := 1
			rightpadding := 2
			count := int(math.Floor(float64(width) * trendpercent))

			redcount := 0
			greencount := 0
			if count == 0 {
				leftpadding = 0
				rightpadding = 4
			} else if count == width {
				leftpadding = 4
				rightpadding = 0
			}

			redcount=width-count-rightpadding
			if redcount < 0 {
				redcount=0
			}
			greencount=count-leftpadding
			greenboxes = strings.Repeat("▓", greencount)
			redboxes = strings.Repeat("▓", redcount)

			// Print Bar
			if tradestats.Number >= 10 {
				return fmt.Sprintf("\r[green]%s[white]%.0f%%[red]%s[white]", greenboxes, trendpercent*100, redboxes)
			}
			return fmt.Sprintf("\rNot enough events")
		}
	}
	return ""
}

// UpdateDetailTable - Prints the detail table based on the input symbol pair
func UpdateDetailTable(symbol string, detail *tview.TextView, stats map[string]Ticker) {
	name := strings.Replace(symbol, "/", "", 1)
	price := stats[name].LastPrice
	volume := stats[name].Volume
	pricechange := stats[name].PriceChangePercent24h
	lowprice := stats[name].LowPrice
	highprice := stats[name].HighPrice
	detail.Clear()
	fmt.Fprintf(detail, "Symbol: %s\nPrice: %s\n24H Change: %s%%\nVolume: %s\nDaily High: %s\nDaily Low:%s",
		name,
		strconv.FormatFloat(price, 'f', -1, 64),
		strconv.FormatFloat(pricechange, 'f', 2, 64),
		strconv.FormatFloat(volume, 'f', -1, 64),
		strconv.FormatFloat(highprice, 'f', -1, 64),
		strconv.FormatFloat(lowprice, 'f', -1, 64))
}

// Basic Function to print the event table first row
func printeventheader(t *tview.Table) {
	// Print Top Row
	row := 0
	cell := tview.NewTableCell("Event").
		SetTextColor(tcell.ColorYellow).
		SetSelectable(false).
		SetAlign(tview.AlignLeft)
	t.SetCell(row, 0, cell)
	// Period
	cell = tview.NewTableCell("Period").
		SetTextColor(tcell.ColorYellow).
		SetSelectable(false).
		SetAlign(tview.AlignRight)
	t.SetCell(row, 1, cell)
	// Asset Pair Symbol
	cell = tview.NewTableCell("Symbol").
		SetTextColor(tcell.ColorYellow).
		SetSelectable(false).
		SetAlign(tview.AlignCenter)
	t.SetCell(row, 2, cell)
	// Amount
	cell = tview.NewTableCell("Amount").
		SetTextColor(tcell.ColorYellow).
		SetSelectable(false).
		SetAlign(tview.AlignCenter)
	t.SetCell(row, 3, cell)
	// Percent
	cell = tview.NewTableCell("Percent").
		SetTextColor(tcell.ColorYellow).
		SetSelectable(false).
		SetAlign(tview.AlignCenter)
	t.SetCell(row, 4, cell)
	// Volume Frequency
	cell = tview.NewTableCell("24H Change").
		SetTextColor(tcell.ColorYellow).
		SetSelectable(false).
		SetAlign(tview.AlignCenter)
	t.SetCell(row, 5, cell)
	// Last Price
	cell = tview.NewTableCell("Price").
		SetTextColor(tcell.ColorYellow).
		SetSelectable(false).
		SetAlign(tview.AlignCenter)
	t.SetCell(row, 6, cell)
}

// PrintEvent - Prints and builds a new event in the event table
func PrintEvent(t *tview.Table, stats map[string]Ticker, ev Event, db *sql.DB) {
	var notice, symbol, period, value string
	var color tcell.Style
	//volfreq := ""
	percent := ""

	lastprice := stats[ev.Data.Symbol].LastPrice
	pricechange := stats[ev.Data.Symbol].PriceChangePercent24h
	if lastprice == 0 || pricechange == 0 {
		stats[ev.Data.Symbol] = GetSymbolTicker(ev.Data.Symbol)
		lastprice = stats[ev.Data.Symbol].LastPrice
		pricechange = stats[ev.Data.Symbol].PriceChangePercent24h
	}
	price := fmt.Sprintf("%v", strconv.FormatFloat(lastprice, 'f', -1, 64))
	change := fmt.Sprintf("%v %%", strconv.FormatFloat(pricechange, 'f', 2, 64))

	// Notice time frame
	switch ev.Data.Period {
	case "DAY_1":
		period = "24H"
	case "WEEK_1":
		period = "7D"
	case "MONTH_1":
		period = "1M"
	case "HOUR_2":
		period = "2h"
	case "MINUTE_15":
		period = "15m"
	case "MINUTE_5":
		period = "5m"
	}

	switch ev.Data.NoticeType {
	case "PRICE_CHANGE":
		notice = "Price Change"
		baseasset := ev.Data.BaseAsset
		symbol = baseasset + "/" + ev.Data.QuotaAsset
		percent = fmt.Sprintf("%.2f%%", ev.Data.PriceChange*100)
		switch ev.Data.EventType {
		case "UP_1":
			color = color.Foreground(tcell.ColorGreen)
		case "DOWN_1":
			color = color.Foreground(tcell.ColorRed)
		case "UP_2":
			color = color.Foreground(tcell.ColorGreen).Underline(true)
		case "DOWN_2":
			color = color.Foreground(tcell.ColorRed).Underline(true)
		case "UP_3":
			color = color.Foreground(tcell.ColorGreen).Bold(true)
		case "DOWN_3":
			color = color.Foreground(tcell.ColorRed).Bold(true)
		}
	case "PRICE_BREAKTHROUGH":
		baseasset := ev.Data.BaseAsset
		symbol = baseasset + "/" + ev.Data.QuotaAsset
		percent = fmt.Sprintf("%.2f%%", ev.Data.PriceChange*100)
		switch ev.Data.EventType {
		case "UP_BREAKTHROUGH":
			color = color.Foreground(tcell.ColorGreen)
			notice = "Price High"
		case "DOWN_BREAKTHROUGH":
			color = color.Foreground(tcell.ColorRed)
			notice = "Price Low"
		}
	case "VOLUME_PRICE":
		baseasset := ev.Data.BaseAsset
		symbol = baseasset + "/" + ev.Data.QuotaAsset
		value = fmt.Sprintf("%.2f", ev.Data.Volume)
		percent = fmt.Sprintf("%.2f", ev.Data.PriceChange*100)
		switch ev.Data.EventType {
		case "HIGH_VOLUME_DROP_1":
			notice = "Large Volume Fall"
			color = color.Foreground(tcell.ColorRed).Underline(true)
		case "HIGH_VOLUME_RISE_1":
			notice = "Large Volume Rise"
			color = color.Foreground(tcell.ColorGreen).Underline(true)
		case "HIGH_VOLUME_DROP_2":
			notice = "Large Volume Fall"
			color = color.Foreground(tcell.ColorRed).Bold(true)
		case "HIGH_VOLUME_RISE_2":
			notice = "Large Volume Rise"
			color = color.Foreground(tcell.ColorGreen).Bold(true)
		case "HIGH_VOLUME_DROP_3":
			notice = "Large Volume Fall"
			color = color.Foreground(tcell.ColorRed).Bold(true).Underline(true)
		case "HIGH_VOLUME_RISE_3":
			notice = "Large Volume Rise"
			color = color.Foreground(tcell.ColorGreen).Bold(true).Underline(true)
		}
	case "BLOCK_TRADE":
		baseasset := ev.Data.BaseAsset
		symbol = baseasset + "/" + ev.Data.QuotaAsset
		value = fmt.Sprintf("%.2f", ev.Data.Volume)
		switch ev.Data.EventType {
		case "BLOCK_TRADES_SELL":
			notice = "Large Sell"
			color = color.Foreground(tcell.ColorRed)
		case "BLOCK_TRADES_BUY":
			notice = "Large Buy"
			color = color.Foreground(tcell.ColorGreen)
		}
	default:
		return
	}

	printeventheader(t)

	// Print last row
	row := t.GetRowCount()
	// Event Name
	cell := tview.NewTableCell(notice).
		SetStyle(color).
		SetSelectable(false).
		SetAlign(tview.AlignLeft)
	t.SetCell(row, 0, cell)
	// Period
	cell = tview.NewTableCell(period).
		SetStyle(color).
		SetSelectable(false).
		SetAlign(tview.AlignCenter)
	t.SetCell(row, 1, cell)
	// Asset Pair Symbol
	cell = tview.NewTableCell(symbol).
		SetStyle(color).
		SetAlign(tview.AlignLeft).
		SetSelectable(true)
	if conf.EnableMouse {
		cell.SetClickedFunc(func() bool {
			asset := strings.Replace(symbol, "/", "_", 1)
			browser.OpenURL(conf.BinanceTerminal + asset)
			return false
		})
	}
	t.SetCell(row, 2, cell)
	// Value + Asset
	cell = tview.NewTableCell(value).
		SetStyle(color).
		SetSelectable(false).
		SetAlign(tview.AlignRight)
	t.SetCell(row, 3, cell)
	// Percent
	cell = tview.NewTableCell(percent).
		SetStyle(color).
		SetSelectable(false).
		SetAlign(tview.AlignRight)
	t.SetCell(row, 4, cell)
	// 24H Change
	cell = tview.NewTableCell(change).
		SetStyle(color).
		SetSelectable(false).
		SetAlign(tview.AlignRight)
	t.SetCell(row, 5, cell)
	// Last Price
	cell = tview.NewTableCell(price).
		SetStyle(color).
		SetSelectable(false).
		SetAlign(tview.AlignRight)
	t.SetCell(row, 6, cell)
}

// PrintTrade - Prints and builds a new trade in the event table
func PrintTrade(t *tview.Table, stats map[string]Ticker, info map[string]Symbol, tr Trade, db *sql.DB) {
	var notice, symbol, period, value, price string
	var color tcell.Style
	percent := ""

	pricechange := stats[tr.Data.Symbol].PriceChangePercent24h
	if pricechange == 0 {
		stats[tr.Data.Symbol] = GetSymbolTicker(tr.Data.Symbol)
		pricechange = stats[tr.Data.Symbol].PriceChangePercent24h
	}
	change := fmt.Sprintf("%v %%", strconv.FormatFloat(pricechange, 'f', 2, 64))
	symbol = info[tr.Data.Symbol].BaseAsset+"/"+info[tr.Data.Symbol].QuoteAsset

	switch tr.Data.EventType {
	case "aggTrade":
		value = fmt.Sprintf("%.2f", tr.Data.Quantity)
		price = fmt.Sprintf("%.2f", tr.Data.Price)
		switch tr.Data.IsMaker {
		case true:
			notice = "Large Maker"
			color = color.Foreground(tcell.ColorYellow)
		case false:
			notice = "Large Taker"
			color = color.Foreground(tcell.ColorBlue)
		}
	default:
		return
	}

	printeventheader(t)
	// Print last row
	row := t.GetRowCount()
	// Event Name
	cell := tview.NewTableCell(notice).
		SetStyle(color).
		SetSelectable(false).
		SetAlign(tview.AlignLeft)
	t.SetCell(row, 0, cell)
	// Period
	cell = tview.NewTableCell(period).
		SetStyle(color).
		SetSelectable(false).
		SetAlign(tview.AlignCenter)
	t.SetCell(row, 1, cell)
	// Asset Pair Symbol
	cell = tview.NewTableCell(symbol).
		SetStyle(color).
		SetAlign(tview.AlignLeft).
		SetSelectable(true)
	if conf.EnableMouse {
		cell.SetClickedFunc(func() bool {
			asset := strings.Replace(symbol, "/", "_", 1)
			browser.OpenURL(conf.BinanceTerminal + asset)
			return false
		})
	}
	t.SetCell(row, 2, cell)
	// Value + Asset
	cell = tview.NewTableCell(value).
		SetStyle(color).
		SetSelectable(false).
		SetAlign(tview.AlignRight)
	t.SetCell(row, 3, cell)
	// Percent
	cell = tview.NewTableCell(percent).
		SetStyle(color).
		SetSelectable(false).
		SetAlign(tview.AlignRight)
	t.SetCell(row, 4, cell)
	// 24H Change
	cell = tview.NewTableCell(change).
		SetStyle(color).
		SetSelectable(false).
		SetAlign(tview.AlignRight)
	t.SetCell(row, 5, cell)
	// Last Price
	cell = tview.NewTableCell(price).
		SetStyle(color).
		SetSelectable(false).
		SetAlign(tview.AlignRight)
	t.SetCell(row, 6, cell)
}

// DisplaySubscribeModal - Modal to subscribe to trades
func DisplaySubscribeModal(tx chan<- string, pages *tview.Pages, s string) {
	// Find Quote Index
	quoteindex:=0
	if strings.Contains(s,"/") {
		for i, v := range conf.Trades.Quotes {
			if v == strings.Split(s, "/")[1] {
				quoteindex=i
				break
			}
		}
	}
	subscribemodal := tview.NewModal().
			SetText("Subscribe To Trades Feed\n\nChoose Quota Asset or Close\n"+strings.Split(s,"/")[0]+"\n").
			AddButtons(append(conf.Trades.Quotes,"Close")).
			SetFocus(quoteindex).
			SetDoneFunc(func(index int, label string){
				if label == "Close" {
					pages.RemovePage("subscribemodal")
				}else {
					SubscribeToTrades(tx, s, label)
					pages.RemovePage("subscribemodal")
				}
			})

	pages.AddPage("subscribemodal",
		subscribemodal,
			false, true)
}

// DisplaySubscribeInputForm - Input form to subscribe to arbitary pair
func DisplaySubscribeInputForm(tx chan<- string, pages *tview.Pages ) {
	form := tview.NewForm()
	form.SetBorder(true).
		SetTitle("Enter an asset to track").
		SetTitleAlign(tview.AlignLeft)
	form.AddInputField("Base Asset", "BTC",10, nil, nil).
			AddButton("Subscribe", func() {
				if s := form.GetFormItemByLabel("Base Asset").(*tview.InputField).GetText(); s != "" {
					DisplaySubscribeModal(tx, pages, s)
				}
				pages.RemovePage("subscribeinput")
			}).
			AddButton("Cancel", func() {
					pages.RemovePage("subscribeinput")
			}).
			SetCancelFunc(func(){
					pages.RemovePage("subscribeinput")
			})

	_, _, screenwidth, screenheight := pages.GetInnerRect()
	form.SetRect((screenwidth-30)/2, (screenheight-10)/2,30,10)
	pages.AddPage("subscribeinput",form, false, true)
}

// DisplayHelpModal - Help screen
func DisplayHelpModal(pages *tview.Pages) {
	helpwidget := tview.NewTextView()
	helpwidget.SetBorder(true).
		SetTitle("Help").
		SetTitleAlign(tview.AlignLeft)
	helpwidget.SetText(messages["helpmodal"]).
			SetDoneFunc(func(key tcell.Key){
				pages.RemovePage("helpmodal")
			})

	_, _, screenwidth, screenheight := pages.GetInnerRect()
	helpwidget.SetRect((screenwidth-60)/2, (screenheight-30)/2,60,20)

	pages.AddPage("helpmodal",
		helpwidget,
		false, true)
}
