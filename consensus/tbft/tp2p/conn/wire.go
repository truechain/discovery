package conn

import (
	amino "github.com/tendermint/go-amino"
	cryptoAmino "truechain/discovery/consensus/tbft/crypto/cryptoamino"
)

var cdc = amino.NewCodec()

func init() {
	cryptoAmino.RegisterAmino(cdc)
	RegisterPacket(cdc)
}
