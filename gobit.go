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

package main

import (
	"fmt"
	"gobit/internal/binance"
	. "gobit/internal/config"
	"gobit/internal/data"
	"gobit/internal/db"
	"gobit/internal/ui"
	"gobit/internal/util"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/net/websocket"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	// Event Channels msg and control
	cws := make(chan string)
	cwc := make(chan bool)

	// Trades Channel msg, control and transmit
	tws := make(chan binance.Trade)
	twc := make(chan bool)
	twtx := make(chan binance.SubChannelMsg)

	// Placeholder vars
	symbolstats := make(map[string]binance.Ticker)
	symbolinfo := make(map[string]data.Symbol)
	quotafilter := ""
	basefilter := ""
	detailstablesymbol := ""
	var percentfilter float32
	var tradestats data.TradeStat

	// Optional log file is stored in the local cache folder
	if Conf.DisableLogging == false {
		logfile, _ := os.OpenFile(Storagepath+"/gobit.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0660)
		log.SetOutput(logfile)
	}

	// TUI init
	app := tview.NewApplication()
	pages := tview.NewPages()
	grid := tview.NewGrid()

	// LiveFeed Widget
	livefeed := ui.InitLiveFeed()
	livefeed.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEscape:
			livefeed.SetSelectable(false, false)
			livefeed.ScrollToEnd()
		case tcell.KeyEnter:
			livefeed.SetSelectable(true, true)
		}
	})

	livefeed.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'o':
			if r, c := livefeed.GetSelectable(); r == true && c == true {
				row, col := livefeed.GetSelection()
				if row != 0 {
					asset := strings.Replace(livefeed.GetCell(row, col).Text, "/", "_", 1)
					util.ShowWebTrade(asset)
				}
			}
		case '\\':
			if r, c := livefeed.GetSelectable(); r == true && c == true {
				row, col := livefeed.GetSelection()
				if row != 0 {
					ui.DisplaySubscribeModal(twtx, pages, livefeed.GetCell(row, col).Text)
				}
			}
		case '/':
			ui.DisplaySubscribeInputForm(twtx, pages)
		case 'u':
			if r, c := livefeed.GetSelectable(); r == true && c == true {
				row, col := livefeed.GetSelection()
				if row != 0 {
					ui.DisplayUnSubscribeModal(twtx, pages, livefeed.GetCell(row, col).Text)
				}
			}
		case 'U':
			ui.DisplayUnSubscribeAllModal(twtx, pages)
		case 'h':
			ui.DisplayHelpModal(pages)
		case 'H':
			ui.DisplayHelpModal(pages)
		}
		return event
	})

	// Trend bar textview widget
	trendbar := ui.InitTrendBar()

	// Momentum Table Widget
	momentumtable := ui.InitMomentumTable()

	// Details Table Widget
	detailstable := ui.InitDetailsTable()

	livefeed.SetSelectedFunc(func(row, column int) {
		cell := livefeed.GetCell(row, column)
		detailstablesymbol = cell.Text
		ui.UpdateDetailTable(cell.Text, detailstable, symbolstats)
	})

	// GUI Grid Layout
	grid.SetRows(9, 3, 0).
		SetColumns(-3, -2)

	// Add items to grid
	grid.AddItem(momentumtable, 0, 0, 1, 1, 0, 0, false)
	grid.AddItem(detailstable, 0, 1, 1, 1, 0, 0, false)
	grid.AddItem(trendbar, 1, 0, 1, 2, 0, 0, false)
	grid.AddItem(livefeed, 2, 0, 1, 2, 2, 10, true)

	// Add grid to pages
	pages.AddPage("grid", grid, true, true)

	// Application Root Config
	app.SetRoot(pages, true).
		SetFocus(grid).
		EnableMouse(true)

	// Database Init
	eventdb, err := db.InitDb(Storagepath + "/event.db")
	if err != nil {
		log.Fatal(err)
	}
	defer eventdb.Close()

	// WebSocket Connections
	// Trade Abnormal Events WebSocket Connection
	go binance.AbnormalEventsWSConn(cwc, cws)

	// Trades WebSocket Connection
	tradesconn, err := websocket.Dial(binance.Tradesurl, "", binance.Tradesurl)
	if err != nil {
		log.Fatal("Unable to open websocket connections")
	}
	// Trades WebSocket Receive
	go binance.TradesWSConnReceive(tradesconn, twc, tws)

	// Trades WebSocket Request
	var subscriptions []string
	go binance.TradesWSConnTransmit(tradesconn, twtx, &subscriptions)

	// Main Goroutine
	go func() {
		// Display Initial Messages
		trendbar.SetTextAlign(tview.AlignCenter)
		trendbar.SetText(data.Messages["notenoughtrades"])
		momentumtable.SetTextAlign(tview.AlignCenter)
		momentumtable.SetText(data.Messages["notenoughdata"])
		detailstable.SetText(data.Messages["details"])

		livefeed.SetCell(0, 0, tview.NewTableCell(data.Messages["waitingfordata"]).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetAlign(tview.AlignCenter))

		// Feed Event Loop
		for {
			select {
			// Event WebSocket Messages
			case m := <-cws:
				ev, err := util.ParseEvent(m)
				if err != nil {
					log.Println("Error parsing msg " + err.Error())
				}
				filter := util.Filter{
					Quote:   quotafilter,
					Base:    basefilter,
					Percent: percentfilter,
				}
				if util.FilterEvent(ev, filter) {
					err := db.InsertDbEvent(ev, eventdb)
					if err != nil {
						log.Println("Error inserting event into db " + err.Error())
					}
					ui.PrintEvent(livefeed, symbolstats, ev, eventdb)
				}
			// Event WebSocket Control
			case control := <-cwc:
				if control {
					break
				}
			// Trades WebSocket Messages
			case tr := <-tws:
				if util.FilterTrade(tr, symbolinfo, symbolstats, &tradestats) {
					err := db.InsertDbTrade(tr, symbolinfo, eventdb)
					if err != nil {
						log.Println("Error inserting trade into db " + err.Error())
					}
					ui.PrintTrade(livefeed, symbolstats, symbolinfo, tr, eventdb)
				}
			// Trades WebSocket Control
			case tcontrol := <-twc:
				if tcontrol {
					break
				}
			}

			// Redraw App
			app.QueueUpdateDraw(func() {
				_, _, trendbarwidth, _ := trendbar.GetInnerRect()
				_, _, momentumtablewidth, _ := momentumtable.GetInnerRect()
				if text := ui.UpdateTrendBar(trendbarwidth, &tradestats); text != "" {
					// Update TrendBar Title with subscriptions
					trendbartitle := ""
					if len(subscriptions) > 0 {
						trendbartitle = " (" + strings.ToUpper(strings.Join(subscriptions, " ")) + ")"
					}
					trendbar.SetTitle("Trade Trend" + trendbartitle)
					trendbar.SetText(text)
				}
				if text := ui.PrintMomentumTable(momentumtablewidth, db.AssetMomentum(eventdb)); text != "" {
					momentumtable.SetTextAlign(tview.AlignRight)
					momentumtable.SetText(text)
				}
			})
			// Warn on small terminals
			if !ui.CheckTermSizeModal(pages) {
				log.Print("Terminal too small")
			}
		}
	}()

	// Periodically fetch asset pairs prices and volumes
	go func() {
		for {
			time.Sleep(Conf.TickerTimer)
			util.FillSymbolStats(symbolstats, eventdb)
			if detailstablesymbol != "" {
				ui.UpdateDetailTable(detailstablesymbol, detailstable, symbolstats)
			}
		}
	}()

	// Run Tui
	if err := app.Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
		fmt.Print("\033\143") // attempt to recover terminal
	}

	os.Exit(0)
}
