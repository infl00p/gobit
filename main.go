/*
	Binance Intelligence Terminal in Go
    Copyright (C) <2021> <infl00p Labs>

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

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
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/websocket"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/jinzhu/configor"
	"github.com/pkg/browser"
	"github.com/rivo/tview"
	"github.com/shibukawa/configdir"
)

// JSON Event Message
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

// Asset Status Data
type AssetStat struct {
	Name      string
	Momentum  float64
	AvgVolume float64
}

// Symbol Ticker Data
type Symbol struct {
	Name                  string `json:"symbol"`
	PriceChangePercent24h string `json:"priceChangePercent"`
	LastPrice             string
	HighPrice             string
	LowPrice              string
	Volume                string
}

// Configuration with default values
var conf = struct {
	BinanceTerminal string        `default:"https://www.binance.com/en/trade/"`
	EnableMouse     bool          `default:"true"`
	TickerTimer     time.Duration `default:"30s"`
	DisableLogging  bool          `default:"false"`
	Db              struct {      // SQL Syntax
		Retention    string `default:"1 hours"`
		SamplePeriod string `default:"10 minutes"`
	}
}{}

// Name
const appname = "gobit"
const vendorname = "infl00pLabs"

// Binance undocumented websocket rest api url for abnormal events
const streamurl = "wss://bstream.binance.com:9443/stream?streams=abnormaltradingnotices"

// Binance Ticker Price Api Endpoint
const tickerurl = "https://api.binance.com/api/v3/ticker/24hr?symbol="

func main() {
	cws := make(chan string)
	cwc := make(chan bool)
	symbolstats := make(map[string]Symbol)
	quotafilter := ""
	basefilter := ""
	detailstablesymbol := ""
	var percentfilter float32

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

	// GUI Layout
	grid := tview.NewGrid().
		SetRows(9, 3, 0).
		SetColumns(-3, -2)

	// LiveFeed Widget
	livefeed := tview.NewTable().
		SetBorders(false).
		SetSeparator(tview.Borders.Vertical).
		SetBordersColor(tcell.ColorGray)
	livefeed.SetBorder(true).SetTitle("Live Feed").
		SetTitleAlign(tview.AlignLeft).
		SetBorderAttributes(tcell.AttrDim)
	livefeed.SetFixed(1, 1).
		SetDoneFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyEscape:
				livefeed.SetSelectable(false, false)
			case tcell.KeyEnter:
				livefeed.SetSelectable(true, true)
			}
		})

	// Trend bar textview widget
	trendbar := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(false)
	trendbar.SetBorder(true).SetTitle("Trend").
		SetTitleAlign(tview.AlignLeft).
		SetBorderAttributes(tcell.AttrDim)
	trendbar.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		return action, nil
	})

	// Momentum Table Widget
	momentumtable := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(false)
	momentumtable.SetBorder(true).SetTitle("Momentum").
		SetTitleAlign(tview.AlignLeft).
		SetBorderAttributes(tcell.AttrDim)
	momentumtable.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		return action, nil
	})

	// Details Table Widget
	detailstable := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(false)
	detailstable.SetBorder(true).SetTitle("Details (" + conf.TickerTimer.String() + ")").
		SetTitleAlign(tview.AlignLeft).
		SetBorderAttributes(tcell.AttrDim)
	detailstable.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		return action, nil
	})

	livefeed.SetSelectedFunc(func(row, column int) {
		cell := livefeed.GetCell(row, column)
		detailstablesymbol = cell.Text
		UpdateDetailTable(cell.Text, detailstable, symbolstats)
	})
	// Add items to grid
	grid.AddItem(momentumtable, 0, 0, 1, 1, 0, 0, false)
	grid.AddItem(detailstable, 0, 1, 1, 1, 0, 0, false)
	grid.AddItem(trendbar, 1, 0, 1, 2, 0, 0, false)
	grid.AddItem(livefeed, 2, 0, 1, 2, 2, 10, true)

	// Database Init
	initdbquery := ""
	rotatequery := ""
	_, err := os.Stat(localcache.Path + "/event.db")
	if os.IsNotExist(err) {
		initdbquery = "create table events(" +
			"timestamp timestamp," +
			"eventtype text," +
			"noticetype text," +
			"symbol text," +
			"baseasset text," +
			"quotaasset text," +
			"volume float," +
			"pricechange float," +
			"period text," +
			"sendtimestamp timestamp)"
	} else {
		// rotate database to keep only recent (retention) data
		rotatequery = "delete from events where " +
			"datetime(timestamp) < datetime('now','-" +
			conf.Db.Retention + "'); vaccum main"
	}

	// Database is stored inside the local cache folder
	eventdb, err := sql.Open("sqlite3", localcache.Path+"/event.db")
	if err != nil {
		log.Fatal(err)
	}
	defer eventdb.Close()
	_, err = eventdb.Exec(initdbquery)
	_, err = eventdb.Exec(rotatequery)

	// WebSocket Connection
	go func() {
		conn, _ := websocket.Dial(streamurl, "", streamurl)
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

	// Feed Event Loop
	go func() {
		// Display Initial Messages
		trendbar.SetTextAlign(tview.AlignCenter)
		trendbar.SetText("Not enough Buy/Sell data")
		momentumtable.SetTextAlign(tview.AlignCenter)
		momentumtable.SetText("Not enough Buy/Sell data")
		detailstable.SetText("Press Enter\nto choose a symbol")

		livefeed.SetCell(0, 0, tview.NewTableCell("Waiting for live data ...").
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetAlign(tview.AlignCenter))

		for {
			select {
			case m := <-cws:
				ev, err := ParseEvent(m)
				if err != nil {
					log.Println("Error parsing msg " + err.Error())
				}
				InsertDbEvent(ev, eventdb)
				if FilterEvent(ev, quotafilter, basefilter, percentfilter) {
					PrintEvent(livefeed, symbolstats, ev, eventdb)
					app.QueueUpdateDraw(func() {
						_, _, trendbarwidth, _ := trendbar.GetInnerRect()
						_, _, momentumtablewidth, _ := momentumtable.GetInnerRect()
						if text := UpdateTrendBar(trendbarwidth, eventdb); text != "" {
							trendbar.SetText(UpdateTrendBar(trendbarwidth, eventdb))
						}
						if text := PrintMomentumTable(momentumtablewidth, AssetMomentum(eventdb)); text != "" {
							momentumtable.SetTextAlign(tview.AlignRight)
							momentumtable.SetText(PrintMomentumTable(momentumtablewidth, AssetMomentum(eventdb)))
						}
					})
				}
			case control := <-cwc:
				if control {
					break
				}
			}
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

	// Application Grid Render
	app.SetRoot(grid, true).
		SetFocus(grid).
		EnableMouse(true)

	if err := app.Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	os.Exit(0)
}

func InsertDbEvent(ev Event, db *sql.DB) error {
	st, err := db.Prepare("insert into events(" +
		"timestamp," +
		"eventtype," +
		"noticetype," +
		"symbol," +
		"baseasset," +
		"quotaasset," +
		"volume," +
		"pricechange," +
		"period," +
		"sendtimestamp" +
		") values(?,?,?,?,?,?,?,?,?,?)")
	defer st.Close()
	if err == nil {
		sendtimestamp := time.Unix(int64(ev.Data.SendTimestamp)/1000,
			1000000*(int64(ev.Data.SendTimestamp)%1000))
		_, err = st.Exec(time.Now(),
			ev.Data.EventType,
			ev.Data.NoticeType,
			ev.Data.Symbol,
			ev.Data.BaseAsset,
			ev.Data.QuotaAsset,
			ev.Data.Volume,
			ev.Data.PriceChange,
			ev.Data.Period,
			sendtimestamp)
	}
	return err
}

func FillSymbolStats(stats map[string]Symbol, db *sql.DB) {
	// Get distinct pairs
	query := "select distinct(symbol) from events " +
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

func GetSymbolTicker(s string) Symbol {
	var sym Symbol
	client := &http.Client{}
	req, err := http.NewRequest("GET", tickerurl+s, nil)
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

func ParseEvent(msg string) (ev Event, err error) {
	err = json.Unmarshal([]byte(msg), &ev)
	return
}

func FilterEvent(ev Event, quotafilter, basefilter string, percentfilter float32) bool {
	if quotafilter == "" && basefilter == "" && percentfilter == 0 {
		return true
	} else {
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
		} else {
			return false
		}
	}
	return true
}

func AssetVolumeFrequency(baseasset string, db *sql.DB) float64 {
	var volfreq sql.NullFloat64
	query := "select (select sum(volume) from events where baseasset ==\"" +
		baseasset + "\" " +
		"and datetime(timestamp) >= datetime('now','-" +
		conf.Db.SamplePeriod + "'))" +
		"/(select sum(volume) from events where baseasset ==\"" +
		baseasset + "\" " +
		"and datetime(timestamp) >= datetime('now','-" +
		conf.Db.Retention + "') " +
		"group by baseasset having count()>10)"
	err := db.QueryRow(query).Scan(&volfreq)
	if err == sql.ErrNoRows || volfreq.Valid != true {
		return 0
	} else if err != nil {
		log.Println("Error executing AssetVolumeFrequency query " + err.Error())
		return 0
	}
	return volfreq.Float64
}

func AssetAvgVolume(baseasset string, db *sql.DB) float64 {
	var volavg sql.NullFloat64
	query := "select avg(volume) from events where baseasset ==\"" +
		baseasset + "\" " +
		"and datetime(timestamp) >= datetime('now','-" +
		conf.Db.Retention + "')"
	err := db.QueryRow(query).Scan(&volavg)
	if err == sql.ErrNoRows || volavg.Valid != true {
		return 0
	} else if err != nil {
		log.Println("Error executing AssetAvgVolume query " + err.Error())
		return 0
	}
	return volavg.Float64
}

func AssetMomentum(db *sql.DB) []AssetStat {
	momentumtable := make([]AssetStat, 0)

	query := "select hourly.baseasset, " +
		"count(tenmin.baseasset)*(sum(distinct(tenmin.volume))/sum(distinct(hourly.volume))) " +
		"as momentum from events as hourly cross join events as tenmin " +
		"on hourly.baseasset == tenmin.baseasset where " +
		"datetime(hourly.timestamp) >= datetime('now','-" +
		conf.Db.Retention + "')" +
		"and datetime(tenmin.timestamp) >= datetime('now','-" +
		conf.Db.SamplePeriod + "')" +
		"and tenmin.noticetype == 'BLOCK_TRADE' and hourly.noticetype == 'BLOCK_TRADE' " +
		"group by hourly.baseasset having count() > 5 order by momentum DESC limit 7;"

	rows, err := db.Query(query)
	if err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		log.Println("Error executing AssetMomentum query " + err.Error())
		return nil
	}
	for rows.Next() {
		var name string
		var momentum sql.NullFloat64
		err = rows.Scan(&name, &momentum)
		if err != nil {
			break
		}
		volumeavg := AssetAvgVolume(name, db)
		momentumtable = append(momentumtable, AssetStat{name, momentum.Float64, volumeavg})
	}
	rows.Close()

	return momentumtable
}

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
				width += 1
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

func UpdateTrendBar(width int, db *sql.DB) string {
	var trendred, trendsum sql.NullFloat64
	var redboxes, greenboxes string

	// Get trend from sql
	query := "select sum(case when eventtype like '%BUY%' then 1 end)," +
		"count(*) from events where noticetype == 'BLOCK_TRADE' " +
		"and datetime(timestamp) >= datetime('now','-" +
		conf.Db.Retention + "')"

	err := db.QueryRow(query).Scan(&trendred, &trendsum)
	if err == sql.ErrNoRows || trendred.Valid != true || trendsum.Valid != true {
	} else if err != nil {
		log.Println("Error executing UpdateTrendBar query " + err.Error())
	} else {
		// Calculate cells based on rounded percentage
		trendpercent := trendred.Float64 / trendsum.Float64
		if width > 3 {
			leftpadding := 1
			rightpadding := 2
			count := int(math.Floor(float64(width) * trendpercent))

			if count == 0 {
				leftpadding = 0
				rightpadding = 4
			} else if count == width {
				leftpadding = 4
				rightpadding = 0
			}
			greenboxes = strings.Repeat("▓", count-leftpadding)
			redboxes = strings.Repeat("▓", width-count-rightpadding)

			// Print Bar
			if trendsum.Float64 >= 5 {
				return fmt.Sprintf("\r[green]%s[white]%.0f%%[red]%s[white]", greenboxes, trendpercent*100, redboxes)
			}
			return fmt.Sprintf("\rNot enough events")
		}
	}
	return ""
}

func UpdateDetailTable(symbol string, detail *tview.TextView, stats map[string]Symbol) {
	name := strings.Replace(symbol, "/", "", 1)
	price, _ := strconv.ParseFloat(stats[name].LastPrice, 64)
	volume, _ := strconv.ParseFloat(stats[name].Volume, 64)
	pricechange, _ := strconv.ParseFloat(stats[name].PriceChangePercent24h, 64)
	lowprice, _ := strconv.ParseFloat(stats[name].LowPrice, 64)
	highprice, _ := strconv.ParseFloat(stats[name].HighPrice, 64)
	detail.Clear()
	fmt.Fprintf(detail, "Symbol: %s\nPrice: %s\n24H Change: %s%%\nVolume: %s\nDaily High: %s\nDaily Low:%s",
		name,
		strconv.FormatFloat(price, 'f', -1, 64),
		strconv.FormatFloat(pricechange, 'f', 2, 64),
		strconv.FormatFloat(volume, 'f', -1, 64),
		strconv.FormatFloat(highprice, 'f', -1, 64),
		strconv.FormatFloat(lowprice, 'f', -1, 64))
}

func PrintEventHeader(t *tview.Table) {
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

func PrintEvent(t *tview.Table, stats map[string]Symbol, ev Event, db *sql.DB) {
	var notice, symbol, period, value string
	var color tcell.Style
	//volfreq := ""
	percent := ""

	lastprice, _ := strconv.ParseFloat(stats[ev.Data.Symbol].LastPrice, 64)
	pricechange, _ := strconv.ParseFloat(stats[ev.Data.Symbol].PriceChangePercent24h, 64)
	if lastprice == 0 || pricechange == 0 {
		stats[ev.Data.Symbol] = GetSymbolTicker(ev.Data.Symbol)
		lastprice, _ = strconv.ParseFloat(stats[ev.Data.Symbol].LastPrice, 64)
		pricechange, _ = strconv.ParseFloat(stats[ev.Data.Symbol].PriceChangePercent24h, 64)
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
	case "BLOCK_TRADE":
		baseasset := ev.Data.BaseAsset
		symbol = baseasset + "/" + ev.Data.QuotaAsset
		value = fmt.Sprintf("%.2f", ev.Data.Volume)
		// volfreq = fmt.Sprintf("%.2f",AssetVolumeFrequency(asset, db) * 100)
		switch ev.Data.EventType {
		case "BLOCK_TRADES_SELL":
			notice = "Sell"
			color = color.Foreground(tcell.ColorRed)
		case "BLOCK_TRADES_BUY":
			notice = "Buy"
			color = color.Foreground(tcell.ColorGreen)
		}
	case "VOLUME_PRICE":
		baseasset := ev.Data.BaseAsset
		symbol = baseasset + "/" + ev.Data.QuotaAsset
		value = fmt.Sprintf("%.2f", ev.Data.Volume)
		percent = fmt.Sprintf("%.2f", ev.Data.PriceChange*100)
		switch ev.Data.EventType {
		case "HIGH_VOLUME_FALL_1":
			notice = "Large Volume Fall"
			color = color.Foreground(tcell.ColorRed).Underline(true)
		case "HIGH_VOLUME_RISE_1":
			notice = "Large Volume Rise"
			color = color.Foreground(tcell.ColorGreen).Underline(true)
		case "HIGH_VOLUME_FALL_3":
			notice = "Large Volume Fall"
			color = color.Foreground(tcell.ColorRed).Bold(true)
		case "HIGH_VOLUME_RISE_3":
			notice = "Large Volume Rise"
			color = color.Foreground(tcell.ColorGreen).Bold(true)
		}
	default:
		return
	}

	PrintEventHeader(t)

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
