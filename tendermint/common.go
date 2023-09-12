package tendermint

import "encoding/hex"

type Hash [32]byte

func (h Hash) String() string {
	return hex.EncodeToString(h[:3])
}
