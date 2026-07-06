package venue

type ID string

const (
	IDPumpfun          ID = "pumpfun"
	IDRaydiumLaunchpad ID = "raydium_launchpad"
	IDMeteoraDBC       ID = "meteora_dbc"
)

func (id ID) String() string { return string(id) }

// Detection holds venue resolved from on-chain pool accounts.
type Detection struct {
	Venue    ID
	BaseMint string
	QNative  string // quote mint for the launchpad pool
	PoolKey  string
}
