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

package db

import (
	"database/sql"
	"gobit/internal/binance"
	. "gobit/internal/config"
	"gobit/internal/data"

	_ "github.com/mattn/go-sqlite3"

	"log"
	"os"
	"time"
)

// InitDb - Inits the database, optionally creates the file
func InitDb(path string) ( eventdb *sql.DB, err error) {
	rotatequery := ""
	initeventsdbquery := "create table if not exists events(" +
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
	inittradesdbquery := "create table if not exists trades(" +
		"timestamp timestamp," +
		"eventtype text," +
		"symbol text," +
		"quoteasset text," +
		"baseasset text," +
		"quantity float," +
		"price float," +
		"tradetimestamp timestamp," +
		"ismaker boolean)"
	_, err = os.Stat(path)
	if os.IsNotExist(err) || Conf.Db.InMemory {
		log.Println("Initializing Database")
	} else {
		// rotate database to keep only recent (retention) data
		log.Println("Rotating Database")
		rotatequery = "delete from events where " +
			"datetime(timestamp) < datetime('now','-" +
			Conf.Db.Retention + "'); delete from trades where " +
			"datetime(timestamp) < datetime('now','-" +
			Conf.Db.Retention + "')"
	}

	// Database is stored inside the local cache folder
	if Conf.Db.InMemory {
		eventdb, err = sql.Open("sqlite3", "file:gobit.db?cache=shared&mode=memory&_busy_timeout=50000000")
	} else {
		eventdb, err = sql.Open("sqlite3", path+"?cache=shared&mode=rwc&_busy_timeout=50000000")
	}
	if err != nil {
		log.Fatal(err)
	}
	_, err = eventdb.Exec(inittradesdbquery)
	if err != nil {
		log.Fatal(err)
	}
	_, err = eventdb.Exec(initeventsdbquery)
	if err != nil {
		log.Fatal(err)
	}
	_, err = eventdb.Exec(rotatequery)
	if err != nil {
		log.Fatal(err)
	}
	_, err = eventdb.Exec("vacuum main")
	if err != nil {
		log.Fatal(err)
	}
	eventdb.SetMaxOpenConns(4)

	return
}

// InsertDbEvent - Gets called when a new event comes
func InsertDbEvent(ev binance.Event, db *sql.DB) error {
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

// InsertDbTrade - Inserts appropriate trade to db
func InsertDbTrade(tr binance.Trade, info map[string]data.Symbol, db *sql.DB) error {
	st, err := db.Prepare("insert into trades(" +
		"timestamp," +
		"eventtype," +
		"symbol," +
		"quoteasset," +
		"baseasset," +
		"quantity," +
		"price," +
		"tradetimestamp," +
		"ismaker" +
		") values(?,?,?,?,?,?,?,?,?)")
	defer st.Close()
	if err == nil {
		tradetimestamp := time.Unix(int64(tr.Data.TradeTimestamp)/1000,
			1000*(int64(tr.Data.TradeTimestamp)%1000))
		_, err = st.Exec(time.Now(),
			tr.Data.EventType,
			tr.Data.Symbol,
			info[tr.Data.Symbol].QuoteAsset,
			info[tr.Data.Symbol].BaseAsset,
			tr.Data.Quantity,
			tr.Data.Price,
			tradetimestamp,
			tr.Data.IsMaker)
	}
	return err
}

// AssetVolumeFrequency - Not used yet
func AssetVolumeFrequency(baseasset string, db *sql.DB) float64 {
	var volfreq sql.NullFloat64
	query := "select (select sum(volume) from events where baseasset ==\"" +
		baseasset + "\" " +
		"and datetime(timestamp) >= datetime('now','-" +
		Conf.Db.SamplePeriod + "'))" +
		"/(select sum(volume) from events where baseasset ==\"" +
		baseasset + "\" " +
		"and datetime(timestamp) >= datetime('now','-" +
		Conf.Db.Retention + "') " +
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

// AssetAvgVolume - Estimate Average Volume
func AssetAvgVolume(baseasset string, db *sql.DB) float64 {
	var volavg sql.NullFloat64
	query := "select avg(volume) from events where baseasset ==\"" +
		baseasset + "\" " +
		"and datetime(timestamp) >= datetime('now','-" +
		Conf.Db.Retention + "')"
	err := db.QueryRow(query).Scan(&volavg)
	if err == sql.ErrNoRows || volavg.Valid != true {
		return 0
	} else if err != nil {
		log.Println("Error executing AssetAvgVolume query " + err.Error())
		return 0
	}
	return volavg.Float64
}

// AssetMomentum - Estimate and build momentum table
func AssetMomentum(db *sql.DB) []data.AssetStat {
	momentumtable := make([]data.AssetStat, 0)

	query := "select h.baseasset, " +
		"count(t.baseasset)*(sum(distinct(t.volume))/sum(distinct(h.volume))) " +
		"as momentum from events as h cross join events as t " +
		"on h.baseasset == t.baseasset where " +
		"datetime(h.timestamp) >= datetime('now','-" +
		Conf.Db.Retention + "')" +
		"and datetime(t.timestamp) >= datetime('now','-" +
		Conf.Db.SamplePeriod + "')" +
		"and t.noticetype == 'BLOCK_TRADE' and h.noticetype == 'BLOCK_TRADE' " +
		"group by h.baseasset having count() > 5 order by momentum DESC limit 7;"

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
		momentumtable = append(momentumtable, data.AssetStat{
			Name: name,
			Momentum: momentum.Float64,
			AvgVolume: volumeavg,
		})
	}
	rows.Close()

	return momentumtable
}
