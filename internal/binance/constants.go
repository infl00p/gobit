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

// Binance undocumented websocket api url for abnormal events
const Noticesurl = "wss://bstream.binance.com:9443/stream?streams=abnormaltradingnotices"

// Binance REST API Endpoint
const Restapiurl = "https://api.binance.com/api/v3/"

// Binance websocket api
const Tradesurl = "wss://stream.binance.com:9443/stream?streams="
