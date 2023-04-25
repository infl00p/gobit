package data

var Messages = map[string]string{
	"details": `Enter: Enable Selection
Enter: Again to show detail
Esc:  Disable Selection
\:    Get Trades of Selection
/:    Input Asset to Subscribe
Ctrl-C: Exit`,
	"notenoughdata":   "Not enough Buy/Sell data",
	"notenoughtrades": "Not enough trades, subscribe to pairs",
	"termsizemodal":   "Too small terminal ...",
	"waitingfordata":  "Waiting for live data ...",
	"helpmodal": `Key shortcuts:
Navigate the live feed table using VIM key shortcuts:
j, Up arrow: Scroll List Up 1 Row
k, Down arrow: Scroll List Down 1 Row
PgUp, Ctrl-B: Scroll List Up By 1 Page
PgDown, Ctrl-F: Scroll List Down By 1 Page
g: Go to Top of Table
G: Go to Bottom of Table
Enter: Enter Selection Mode
Enter: In Selection Mode select symbol to show details
\:	In Selection Mode, subscribe symbol to trades feed
o: In Selection Mode, Launch Web Trade Page
Esc: Exit Selection Mode
/: Display Input Form to subscribe a symbol to trades feed
u: Unsubscribe pair from trades feed
U: Unsubscribe all pairs from trades feed
h, H: Display this Help Modal
Ctrl-C: quit program
`,
}
