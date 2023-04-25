# Binance Intelligence terminal

Warning: The app is still on alpha stage

Intro
---
gobit is a TUI application to interface with Binance, using an undocumented
and public Websocket API to fetch abnormal events, like large trades (Buy and
Sells over 30k USD - no longer working) large price changes and large volume changes.

These information can be useful for a small trader that wants to monitor what
the whales are doing or wants to catch pump or dump events (sudden rise or fall).

Additionally the user has the option to subscribe to the aggregated trade feed of
pairs, to monitor large maker or taker trades. This feature uses the documented
websocket API and can be quite resource intensive.

The application fetches events and writes them in a sqlite database file inside
the user os cache directory. Additionally logging is enabled by default and all
information messages and errors are written to a event.log file  inside the os
cache directory also.

UI
---
The TUI displays a grid of four (4) widgets, with the main being the live feed
of events as they come from Binance. The Trend widget is a bar which displays
the percentage of all Maker and Taker trades and can be an indicator of the
destination the market is heading. The Popularity widget is a ranking of the
most traded assets (not pair) by whales, during the last period (by default 10
minutes). The details widget displays additional information when a pair symbol
is selected (using Enter key).

See gobit-scr.png for a screen shot of the TUI in action.

Key shortcuts
---
Navigate the live feed table using VIM key shortcuts:
* j, k, Up arrow, Down arrow: Down and Up by on row
* PgUp, PgDown, Ctrl-B, Ctrl-F: Page Up and Down 
* g, G: Go to Top or bottom
* h, H: Show Help
* Ctrl-C: quit program

Selection Mode
* Enter: Enter Selection Mode
* Esc: Exit Selection Mode
* Enter: In Selection Mode, Show Details of Pair

Trade Feed
* \\: Subscribe to trades of selected pair
* /: Select a pair to subscribe

Mouse action
---
By clicking a symbol pair in the livefeed widget you can launch the pair's trade
web page in your browser 

Config file
---
Configuration is stored on your os configuration directory usually as config.json
See internal/config/config.go source file for an example config file

News
---
- Binance has permanently disabled the Large Trade feeds on this API
- As of 2021-07-08 Binance has restored Large Trade feed on this API
- As of 2021-06-08 Binance has muted Large Trades on this API

Donations/Tips
---
* DogeCoin: DN5b9sknM8Hx312PhemcGMWEJNNKDuR3ro
* Litecoin: LfxV3vjZtLHhywjvmWcqF3boDmazdhCdao
* Ethereum: 0x7E7a779aa2223ada83712b6F875E19046F4dd949
* BCH:      qzxap5rvzcezrplt59y5m9ztsx0aun2r8gm594l38w
* Monero:   437wCvFAYUUUTAzKqoixx6dPagwek9xKw4he6JGNoEBj5xBRg2mayZMHkrbzXbKQrrgZqwyqc6m9AXTTHguPqJxaLDqTXv9

License
---
    Copyright (C) <2021-2023>  <infl00p Labs>

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
