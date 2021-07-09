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
	"golang.org/x/net/websocket"
	"io"
	"log"
	"fmt"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jinzhu/configor"
	"github.com/rivo/tview"
	"github.com/shibukawa/configdir"
)

// Event Message
// JSON Structure
type Event struct {
	Stream string
	Data   struct {
		EventType     string
		NoticeType    string
		Symbol        string
		BaseAsset     string
		QuotaAsset    string
		Volume        float32 `json:"volume"`
		PriceChange   float32 `json:"priceChange"`
		Period        string
		SendTimestamp uint64
	}
}

// Ticker Symbol Data
// JSON Structure
type Ticker struct {
	Name                  string `json:"symbol"`
	PriceChangePercent24h float64 `json:"priceChangePercent,string"`
	LastPrice             float64 `json:"lastPrice,string"`
	HighPrice             float64 `json:"highPrice,string"`
	LowPrice              float64 `json:"lowPrice,string"`
	Volume                float64 `json:"volume,string"`
}

// Trade Websocket receive
// JSON Structure
type Trade struct {
	Result string
	ID	uint64
	Stream string
	Data   struct {
		EventType     string `json:"e"`
		Symbol        string `json:"s"`
		Quantity        float64 `json:"q,string"`
		Price        float64 `json:"p,string"`
		EventTimestamp uint64 `json:"E"`
		TradeTimestamp uint64 `json:"T"`
		TradeID		uint64 `json:"a"`
		IsMaker		bool `json:"m"`
		Ignore		bool `json:"M"`
	}
}

// SubscribeRequest Websocket subscription
// JSON Structure
type SubscribeRequest struct {
	Method	string `json:"method"`
	ID	uint64	`json:"id"`
	Params []string `json:"params"`
}

// AssetStat Status Data
type AssetStat struct {
	Name      string
	Momentum  float64
	AvgVolume float64
}

// Symbol Asset Data
type Symbol struct {
	Symbol	string
	BaseAsset	string
	QuoteAsset	string
}

// TradeStat statistics data
type TradeStat struct {
	Taker	float64
	Maker	float64
	Number	uint64
}

var messages = map[string]string {
	"details":
`Enter: Enable Selection
Enter: Again to show detail
Esc:  Disable Selection
\:    Get Trades of Selection
/:    Input Asset to Subscribe
Ctrl-C: Exit`,
	"notenoughdata": "Not enough Buy/Sell data",
	"notenoughtrades": "Not enough trades, subscribe to pairs",
	"waitingfordata": "Waiting for live data ...",
	"helpmodal":
`Key shortcuts:
Navigate the live feed table using VIM key shortcuts:
j, Up arrow: Scroll List Up 1 Row
k, Down arrow: Scroll List Down 1 Row
PgUp, Ctrl-B: Scroll List Up By 1 Page
PgDown, Ctrl-F: Scroll List Down By 1 Page
g: Go to Top of Table
G: Go to Bottom of Table
Enter: Enter Selection Mode
Enter: In Selection Mode select Symbol Pair to show details
\:	In Selection Mode, subscribe pair to trades feed
Esc: Exit Selection Mode
/: Display Input Form to subscribe a pair to trades feed
h, H: Display this Help Modal
Ctrl-C: quit program
`,
}

// Configuration with default values
// Equivalent Json config file:
// {
// 	"BinanceTerminal": "https://www.binance.com/en/trade/",
// 	"EnableMouse":	"true",
// 	"DisableTimer":	"30s",
// 	"DisableLogging":	"false",
// 	"Db" : {
// 		"Retention": "1 hours",
// 		"SamplePeriod": "10 minutes"
// 	}
// 	"Trades" : {
//		"Quotes": ["USDT", "BTC", "BNB", "ETH"],
// 		"DefaultQuote": "USDT",
// 		"Threshhold": 50000
// 	}
//
var conf = struct {
	BinanceTerminal string        `default:"https://www.binance.com/en/trade/"`
	EnableMouse     bool          `default:"true"`
	TickerTimer     time.Duration `default:"30s"`
	DisableLogging  bool          `default:"false"`
	Db              struct {      // SQL Syntax
		Retention    string `default:"1 hours"`
		SamplePeriod string `default:"10 minutes"`
	}
	Trades			struct {
		Quotes []string `default:"[USDT, BTC, BNB, ETH]"`
		DefaultQuote		string	`default:"USDT"`
		Threshhold	float64	`default:"50000"`
	}
}{}

// App Name
const appname = "gobit"
const vendorname = "infl00pLabs"

// Binance undocumented websocket api url for abnormal events
const noticesurl = "wss://bstream.binance.com:9443/stream?streams=abnormaltradingnotices"

// Binance REST API Endpoint
const restapiurl = "https://api.binance.com/api/v3/"

// Binance websocket api
const tradesurl = "wss://stream.binance.com:9443/stream?streams="

func main() {
	// Event Channels msg and control
	cws := make(chan string)
	cwc := make(chan bool)

	// Trades Channel msg, control and transmit
	tws := make(chan Trade)
	twc := make(chan bool)
	twtx := make(chan string)

	// Placeholder vars
	symbolstats := make(map[string]Ticker)
	symbolinfo := make(map[string]Symbol)
	quotafilter := ""
	basefilter := ""
	detailstablesymbol := ""
	var percentfilter float32
	var tradestats TradeStat

	// Configuration Init
	configdirs := configdir.New(vendorname, appname)
	localcache := configdirs.QueryCacheFolder()
	localconfig := configdirs.QueryFolderContainsFile("config.json")
	localcache.MkdirAll()

	// Load Config and configuration file
	if localconfig != nil {
		err := configor.Load(&conf, localconfig.Path+"/config.json")
		if err != nil {
			log.Fatal(err.Error())
			os.Exit(1)
		}
	} else {
		err := configor.Load(&conf, "config.json")
		if err != nil {
			log.Fatal(err.Error())
			os.Exit(1)
		}
	}

	// Optional log file is stored in the local cache folder
	if conf.DisableLogging == false {
		logfile, _ := os.OpenFile(localcache.Path+"/event.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0660)
		log.SetOutput(logfile)
	}

	// TUI init
	app := tview.NewApplication()
	pages := tview.NewPages()
	grid := tview.NewGrid()

	// LiveFeed Widget
	livefeed := InitLiveFeed()
	livefeed.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEscape:
			livefeed.SetSelectable(false, false)
			livefeed.ScrollToEnd()
		case tcell.KeyEnter:
			livefeed.SetSelectable(true, true)
		}
	})

	livefeed.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey{
		switch event.Rune() {
		case '\\':
			if r,c := livefeed.GetSelectable(); r == true && c == true {
				row, col := livefeed.GetSelection()
				if row != 0 {
					DisplaySubscribeModal(twtx, pages, livefeed.GetCell(row, col).Text)
				}
			}
		case '/':
			DisplaySubscribeInputForm(twtx, pages)
		case 'h':
			DisplayHelpModal(pages)
		case 'H':
			DisplayHelpModal(pages)
		}
		return event
	})

	// Trend bar textview widget
	trendbar := InitTrendBar()

	// Momentum Table Widget
	momentumtable := InitMomentumTable()

	// Details Table Widget
	detailstable := InitDetailsTable()

	livefeed.SetSelectedFunc(func(row, column int) {
		cell := livefeed.GetCell(row, column)
		detailstablesymbol = cell.Text
		UpdateDetailTable(cell.Text, detailstable, symbolstats)
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
	eventdb, err := InitDb(localcache.Path+"/event.db")
	if err != nil {
		log.Fatal(err)
	}
	defer eventdb.Close()

	// Abnormal Events WebSocket Connection
	go func() {
		conn, _ := websocket.Dial(noticesurl, "", noticesurl)
		for {
			var msg string
			err := websocket.Message.Receive(conn, &msg)
			if err != nil {
				if err == io.EOF {
					cwc <- true
					return
				}
				log.Println("Error receiving msg " + err.Error())
				cwc <- true
			}
			if conf.DisableLogging == false {
				log.Println(msg)
			}
			cws <- msg
		}
	}()

	// Trades WebSocket Connection
	tradesconn, err := websocket.Dial(tradesurl, "", tradesurl)
	if err != nil {
		log.Fatal("Unable to open websocket connections")
	}

	// Trades WebSocket Receive
	go func() {
		for {
			var tmsg Trade
			err := websocket.JSON.Receive(tradesconn, &tmsg)
			if err != nil {
				if err == io.EOF {
					twc <- true
					log.Println("End of File msg received " + err.Error())
					return
				}
				log.Println("Error receiving trades msg " + err.Error())
				twc <- true
			}
			// Trade messages lack id field
			if tmsg.ID == 0 {
				tws <- tmsg
			}
		}
	}()

	// Trades WebSocket Request
	go func() {
		count := 1
		for {
			if v := <-twtx ; v != "" {
				var s SubscribeRequest
				// Subscribe to stream
				s.Method = "SUBSCRIBE"
				s.Params = []string{v}
				s.ID = uint64(count)
				log.Println(s)
				websocket.JSON.Send(tradesconn, s)
				count++
			}
		}
	}()

	// Main Goroutine
	go func() {
		// Display Initial Messages
		trendbar.SetTextAlign(tview.AlignCenter)
		trendbar.SetText(messages["notenoughtrades"])
		momentumtable.SetTextAlign(tview.AlignCenter)
		momentumtable.SetText(messages["notenoughdata"])
		detailstable.SetText(messages["details"])

		livefeed.SetCell(0, 0, tview.NewTableCell(messages["waitingfordata"]).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetAlign(tview.AlignCenter))

		// Feed Event Loop
		for {
			select {
			// Event WebSocket Messages
			case m := <-cws:
				ev, err := ParseEvent(m)
				if err != nil {
					log.Println("Error parsing msg " + err.Error())
				}
				if FilterEvent(ev, quotafilter, basefilter, percentfilter) {
					err := InsertDbEvent(ev, eventdb)
					if err != nil {
						log.Println("Error inserting event into db " + err.Error())
					}
					PrintEvent(livefeed, symbolstats, ev, eventdb)
				}
			// Event WebSocket Control
			case control := <-cwc:
				if control {
					break
				}
			// Trades WebSocket Messages
			case tr := <-tws:
				if FilterTrade(tr, symbolinfo, symbolstats, &tradestats) {
					err := InsertDbTrade(tr, symbolinfo, eventdb)
					if err != nil {
						log.Println("Error inserting trade into db " + err.Error())
					}
					PrintTrade(livefeed,symbolstats,symbolinfo,tr,eventdb)
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
				if text := UpdateTrendBar(trendbarwidth, &tradestats); text != "" {
					trendbar.SetText(UpdateTrendBar(trendbarwidth, &tradestats))
				}
				if text := PrintMomentumTable(momentumtablewidth, AssetMomentum(eventdb)); text != "" {
					momentumtable.SetTextAlign(tview.AlignRight)
					momentumtable.SetText(PrintMomentumTable(momentumtablewidth, AssetMomentum(eventdb)))
				}
			})
		}
	}()

	// Periodically fetch asset pairs prices and volumes
	go func() {
		for {
			time.Sleep(conf.TickerTimer)
			FillSymbolStats(symbolstats, eventdb)
			if detailstablesymbol != "" {
				UpdateDetailTable(detailstablesymbol, detailstable, symbolstats)
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
