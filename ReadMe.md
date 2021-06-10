# Binance Intelligence terminal

Warning: The app is still on alpha stage.
- As of 2021-06-08 Binance has muted Large Trades on this API

Intro
---
gobit is a TUI application to interface with Binance, using a undocumented and public Websocket API and fetch abnormal events, like large trades (Buy and Sells over 30k USD) large price changes and large volume changes.

These information can be useful for a small trader that wants to monitor what the whales are doing or wants to catch pump or dump events (sudden rise or fall).

The application fetches events and writes them in a sqlite database file inside the user os cache directory. Additionally logging is enabled by default and all information messages and errors are written to a event.log file  inside the os cache directory also.

UI
---
The TUI displays a grid of four (4) widgets, with the main being the live feed of events as they come from Binance. The Trend widget is a bar which displays the percentage of all Buys vs Sells and can be an indicator of the destination the market is heading. The Momentum widget is a ranking of the most traded assets (not pair) by whales, during the last period (by default 10 minutes). The details widget displays additional information when a pair symbol is selected (using Enter key).

Key shortcuts
---
Navigate the live feed table using VIM key shortcuts:
j, k, Up arrow, Down arrow: Down and Up by on row
PgUp, PgDown, Ctrl-B, Ctrl-F: Page Up and Down 
g, G: Go to Top or bottom
Enter: Enter Selection Mode
Esc: Exit Selection Mode
Ctrl-C: quit program


Mouse action
---
By clicking a symbol pair in the livefeed widget you can launch the pair's trade web page in your browser 

Config file
---
Configuration is stored on your os configuration directory usually as config.json

Donations/Tips
---
* DogeCoin: DN5b9sknM8Hx312PhemcGMWEJNNKDuR3ro
* Litecoin: LfxV3vjZtLHhywjvmWcqF3boDmazdhCdao
* Ethereum: 0x7E7a779aa2223ada83712b6F875E19046F4dd949
* BCH:      qzxap5rvzcezrplt59y5m9ztsx0aun2r8gm594l38w
* Monero:   437wCvFAYUUUTAzKqoixx6dPagwek9xKw4he6JGNoEBj5xBRg2mayZMHkrbzXbKQrrgZqwyqc6m9AXTTHguPqJxaLDqTXv9

License
---
    Copyright (C) <2021>  <infl00p Labs>

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
