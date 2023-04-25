package data

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
