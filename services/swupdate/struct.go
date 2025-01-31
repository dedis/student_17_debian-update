package swupdate

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/cothority/services/timestamp"
	"github.com/satori/go.uuid"
)

func init() {
	for _, msg := range []interface{}{
		Policy{},
		Release{},
		storage{},
		SwupChain{},
		LatestBlocksRet{},
		LatestBlocksRetInternal{},
		TimestampRets{},
	} {
		network.RegisterPacketType(msg)
	}
}

type Policy struct {
	Name    string
	Version string
	// Represents how to fetch the source of that version -
	// only implementation so far will be deb-src://, but github://
	// and others are possible.
	Source     string
	Keys       []string
	Threshold  int
	BinaryHash string
	SourceHash string
}

func NewPolicy(str string) (*Policy, error) {
	p := &Policy{}
	_, err := toml.Decode(str, p)
	return p, err
}

// Niktin calls this 'Snapshot'
type Release struct {
	Policy      *Policy
	Signatures  []string
	VerifyBuild bool
}

type SwupChain struct {
	Root    *skipchain.SkipBlock
	Data    *skipchain.SkipBlock
	Release *Release
}

// Timestamp contains the information generated by the swupdate service
// regarding the timestamp of the latests blocks of all skipchains the service
// handle.
type Timestamp struct {
	// the "proof" field is never set because it's something to pass according
	// to each client request
	timestamp.SignatureResponse

	Proofs []crypto.Proof
}

type ProjectID uuid.UUID

type CreatePackage struct {
	Roster  *sda.Roster
	Release *Release
	Base    int
	Height  int
}

type CreatePackageRet struct {
	SwupChain *SwupChain
}

type UpdatePackage struct {
	SwupChain *SwupChain
	Release   *Release
}

type UpdatePackageRet struct {
	SwupChain *SwupChain
}

// PackageSC searches for the skipchain responsible for the PackageName.
type PackageSC struct {
	PackageName string
}

// If no skipchain for PackageName is found, both first and last are nil.
// If the skipchain has been found, both the genesis-block and the latest
// block will be returned.
type PackageSCRet struct {
	First *skipchain.SkipBlock
	Last  *skipchain.SkipBlock
}

// Request skipblocks needed to get to the latest version of the package.
// LastKnownSB is the latest skipblock known to the client.
type LatestBlock struct {
	LastKnownSB skipchain.SkipBlockID
}

// Similar to LatestBlock but asking update information for all blocks being
// managed by the service.
type LatestBlocks struct {
	LastKnownSBs []skipchain.SkipBlockID
}

// Returns the timestamp of the latest skipblock, together with an eventual
// shortes-link of skipblocks needed to go from the LastKnownSB to the
// current skipblock.
type LatestBlockRet struct {
	Timestamp *Timestamp
	Update    []*skipchain.SkipBlock
}

// Similar to LatestBlockRet but gives information on *all* packages
type LatestBlocksRet struct {
	Timestamp *Timestamp
	// Each updates for each packages ordered in same order that in LatestBlocks
	Updates [][]*skipchain.SkipBlock
}

// Internal structure with lengths
type LatestBlocksRetInternal struct {
	Timestamp *Timestamp
	// Each updates for each packages ordered in same order that in LatestBlocks
	Updates []*skipchain.SkipBlock
	// STUPID: [][] is not correctly parsed by protobuf, so use lengths...
	Lengths []int64
}

// TimestampRequest asks the swupdate service to give back the proof of
// inclusion for the latest timestamp merkle tree including the package
// denoted by Name.
type TimestampRequest struct {
	Name string
}

// Similar to TimestampRequest but asking more multiple proof at the same time
type TimestampRequests struct {
	Names []string
}

// Returns the Proofs to use to verify the inclusion of the package given in
// TimestampRequest
type TimestampRet struct {
	Proof crypto.Proof
}

// Similar to TimestampRet but returns the requested proofs designated by
// package names.
type TimestampRets struct {
	Proofs map[string]crypto.Proof
}
