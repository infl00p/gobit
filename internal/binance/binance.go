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

package binance

import (
	"encoding/json"
	. "gobit/internal/config"
	"gobit/internal/data"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"golang.org/x/net/websocket"
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
	Name                  string  `json:"symbol"`
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
	ID     uint64
	Stream string
	Data   struct {
		EventType      string  `json:"e"`
		Symbol         string  `json:"s"`
		Quantity       float64 `json:"q,string"`
		Price          float64 `json:"p,string"`
		EventTimestamp uint64  `json:"E"`
		TradeTimestamp uint64  `json:"T"`
		TradeID        uint64  `json:"a"`
		IsMaker        bool    `json:"m"`
		Ignore         bool    `json:"M"`
	}
}

// SubChannelMsg Subcription Channel Message
type SubChannelMsg struct {
	Method     string
	StreamName string
}

// SubscribeRequest Websocket subscription
// JSON Structure
type SubscribeRequest struct {
	Method string   `json:"method"`
	ID     uint64   `json:"id"`
	Params []string `json:"params"`
}

// Trade Abnormal Events WebSocket Connection
func AbnormalEventsWSConn(cwc chan bool, cws chan string) {
	conn, _ := websocket.Dial(Noticesurl, "", Noticesurl)
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
		if Conf.DisableLogging == false {
			log.Println(msg)
		}
		cws <- msg
	}
}

// Trades WebSocket Receive
func TradesWSConnReceive(tradesconn *websocket.Conn, twc chan bool, tws chan Trade) {
	for {
		var tmsg Trade
		err := websocket.JSON.Receive(tradesconn, &tmsg)
		if err != nil {
			if err == io.EOF {
				log.Println("End of File msg received " + err.Error())
			} else {
				log.Println("Error receiving trades msg " + err.Error())
				twc <- true
			}
		}
		// Trade messages lack id field
		if tmsg.ID == 0 {
			tws <- tmsg
		}
	}
}

func TradesWSConnTransmit(tradesconn *websocket.Conn, twtx chan SubChannelMsg, subscriptions *[]string) {
	count := 1
	for {
		if v := <-twtx; v.StreamName != "" {
			var s SubscribeRequest
			switch v.Method {
			case "Subscribe":
				// Subscribe to stream
				s.Method = "SUBSCRIBE"
				stream := strings.ToLower(v.StreamName) + "@aggTrade"
				s.Params = []string{stream}
				s.ID = uint64(count)

				// Check if already subscribed
				check := true
				for _, w := range *subscriptions {
					if w == v.StreamName {
						check = false
						break
					}
				}
				if check {
					*subscriptions = append(*subscriptions, v.StreamName)
				}
			case "Unsubscribe":
				// Unsubscribe from stream
				s.Method = "UNSUBSCRIBE"
				stream := strings.ToLower(v.StreamName) + "@aggTrade"
				s.Params = []string{stream}
				s.ID = uint64(count)
				// Remove Subscription from slice
				for i, w := range *subscriptions {
					if w == v.StreamName {
						*subscriptions = append((*subscriptions)[:i], (*subscriptions)[i+1:]...)
						break
					}
				}
			case "UnsubscribeAll":
				if len(*subscriptions) == 0 {
					continue
				}
				// Unsubscribe All Subscriptions
				s.Method = "UNSUBSCRIBE"
				for _, k := range *subscriptions {
					s.Params = append(s.Params, strings.ToLower(k)+"@aggTrade")
				}
				s.ID = uint64(count)
				*subscriptions = nil
			}
			if Conf.DisableLogging == false {
				log.Println(s)
			}

			// Send stream to websocket interface
			websocket.JSON.Send(tradesconn, s)
			count++
		}
	}
}

// WsAbnormalEvents
// Abnormal Events WebSocket Connection
func WsAbnormalEvents(cws chan string, cwc chan bool) {
	go func() {
		conn, err := websocket.Dial(Noticesurl, "", Noticesurl)
		if err != nil {
			cwc <- true
			return
		}
		for {
			var msg string
			err = websocket.Message.Receive(conn, &msg)
			if err != nil {
				if err == io.EOF {
					cwc <- true
					return
				}
				log.Println("Error receiving msg " + err.Error())
				cwc <- true
			}
			if Conf.DisableLogging == false {
				log.Println(msg)
			}
			cws <- msg
		}
	}()
}

// WsTradesAPI
func WsTradesAPI(tws chan Trade, twc chan bool) (*websocket.Conn, error) {
	// Trades WebSocket Connection
	conn, err := websocket.Dial(Tradesurl, "", Tradesurl)
	if err != nil {
		log.Println("Unable to open websocket connections")
		return conn, err
	}

	// Trades WebSocket Receive
	go func() {
		for {
			var tmsg Trade
			err := websocket.JSON.Receive(conn, &tmsg)
			if err != nil {
				if err == io.EOF {
					log.Println("End of File msg received " + err.Error())
				} else {
					log.Println("Error receiving trades msg " + err.Error())
					twc <- true
				}
			}
			// Trade messages lack id field
			if tmsg.ID == 0 {
				tws <- tmsg
			}
		}
	}()
	return conn, err
}

// WsTradesControl
// WebSocket control and update of subscription slice
func WsTradesControl(twtx <-chan SubChannelMsg, tradesconn *websocket.Conn) **([]string) {
	subscriptions := make([]string, 0)
	subp := &subscriptions

	go func() {
		count := 1
		for {
			if v := <-twtx; v.StreamName != "" {
				var s SubscribeRequest
				switch v.Method {
				case "Subscribe":
					// Subscribe to stream
					s.Method = "SUBSCRIBE"
					stream := strings.ToLower(v.StreamName) + "@aggTrade"
					s.Params = []string{stream}
					s.ID = uint64(count)

					// Check if already subscribed
					check := true
					for _, w := range subscriptions {
						if w == v.StreamName {
							check = false
							break
						}
					}
					if check {
						subscriptions = append(subscriptions, v.StreamName)
					}
				case "Unsubscribe":
					// Unsubscribe from stream
					s.Method = "UNSUBSCRIBE"
					stream := strings.ToLower(v.StreamName) + "@aggTrade"
					s.Params = []string{stream}
					s.ID = uint64(count)
					// Remove Subscription from slice
					for i, w := range subscriptions {
						if w == v.StreamName {
							subscriptions = append((subscriptions)[:i], (subscriptions)[i+1:]...)
							break
						}
					}
				case "UnsubscribeAll":
					if len(subscriptions) == 0 {
						continue
					}
					// Unsubscribe All Subscriptions
					s.Method = "UNSUBSCRIBE"
					for _, k := range subscriptions {
						s.Params = append(s.Params, strings.ToLower(k)+"@aggTrade")
					}
					s.ID = uint64(count)
					subscriptions = nil
				}
				if Conf.DisableLogging == false {
					log.Println(s)
				}

				// Send stream to websocket interface
				websocket.JSON.Send(tradesconn, s)
				count++
			}
		}
	}()
	return &subp
}

// GetSymbolTicker returns Ticker structure
// Rest API call to get ticker information about a specific
// symbol pair
func GetSymbolTicker(s string) Ticker {
	var sym Ticker
	client := &http.Client{}
	req, err := http.NewRequest("GET", Restapiurl+"ticker/24hr?symbol="+s, nil)
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
func GetSymbolInfo(s string, m map[string]data.Symbol) error {
	type ExchangeInfo struct {
		Symbols []data.Symbol `json:"symbols"`
	}

	sym := strings.ToUpper(s)
	log.Println("Getting Info for ", sym, s)
	var info ExchangeInfo
	client := &http.Client{}
	req, err := http.NewRequest("GET", Restapiurl+"exchangeInfo?symbol="+sym, nil)
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
