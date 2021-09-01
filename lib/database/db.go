package database

import (
	"crypto"
	"encoding/binary"
	"io"
	"math"
	"math/rand"
	"time"

	"github.com/cloudflare/circl/group"
	"github.com/ldsec/lattigo/v2/bfv"
	"github.com/nikirill/go-crypto/openpgp/packet"
	"github.com/si-co/vpir-code/lib/field"
	"github.com/si-co/vpir-code/lib/utils"
	"golang.org/x/crypto/blake2b"
)

type DB struct {
	KeysInfo []*KeyInfo
	Entries  []uint32

	Info
}

type KeyInfo struct {
	// subset of openpgp.PublicKey
	CreationTime time.Time
	PubKeyAlgo   packet.PublicKeyAlgorithm
	KeyId        uint64
	BlockLength  int // length of data in blocks defined in number of elements

}

type Info struct {
	NumRows    int
	NumColumns int
	BlockSize  int
	// TODO: we really need this? Seems like not since set to uint64 by default
	// IdentifierLength int // length of each identifier in bytes
	// BlockLengths  []int // length of data in blocks defined in number of elements

	// PIR type: classical, merkle, signature
	PIRType string

	*Auth
	*Merkle

	// Lattice parameters for the single-server data retrieval
	LatParams *bfv.Parameters
}

// Auth is authentication information for the single-server setting
type Auth struct {
	// The global digest that is a hash of all the row digests. Public.
	Digest []byte
	// One digest per row, authenticating all the elements in that row.
	SubDigests []byte
	// ECC group and hash algorithm used for digest computation and PIR itself
	Group group.Group
	Hash  crypto.Hash
	// Due to lack of the size functions in the lib API, we store it in the db info
	ElementSize int
	ScalarSize  int
}

// Merkle is the info needed for the Merkle-tree based approach
type Merkle struct {
	Root     []byte
	ProofLen int
}

func NewEmptyDB(info Info) (*DB, error) {
	return &DB{
		Info:     info,
		KeysInfo: make([]*KeyInfo, 0),
		Entries:  make([]uint32, 0),
	}, nil
}

func NewInfo(nRows, nCols, bSize int) Info {
	return Info{
		NumRows:      nRows,
		NumColumns:   nCols,
		BlockSize:    bSize,
		BlockLengths: make([]int, nRows*nCols),
	}
}

func CreateRandomDB(rnd io.Reader, numIdentifiers int) (*DB, error) {
	rand.Seed(time.Now().UnixNano())
	entryLength := 64

	// create random keys
	// for random db use 2048 bits = 64 uint32 elements
	entries := field.RandVectorWithPRG(numIdentifiers*entryLength, rnd)

	keysInfo := make([]*KeyInfo, numIdentifiers)
	for i := 0; i < numIdentifiers; i++ {
		// random creation date
		ct := utils.Randate()

		// random algorithm, taken from random permutation of
		// https://pkg.go.dev/golang.org/x/crypto/openpgp/packet#PublicKeyAlgorithm
		algorithms := []packet.PublicKeyAlgorithm{1, 16, 17, 18, 19}
		pka := algorithms[rand.Intn(len(algorithms))]

		// random id
		id := rand.Uint64()

		// in this case lengths are all equal, 2048 bits = 64 uint32 elements
		bl := entryLength

		keysInfo[i] = &KeyInfo{
			CreationTime: ct,
			PubKeyAlgo:   pka,
			KeyId:        id,
			BlockLength:  bl,
		}

	}

	// in this case lengths are all equal
	info := NewInfo(1, numIdentifiers, entryLength)

	return &DB{
		KeysInfo: keysInfo,
		Entries:  entries,
		Info:     info,
	}, nil
}

// HashToIndex hashes the given id to an index for a database of the given
// length
func HashToIndex(id string, length int) int {
	hash := blake2b.Sum256([]byte(id))
	return int(binary.BigEndian.Uint64(hash[:]) % uint64(length))
}

func CalculateNumRowsAndColumns(numBlocks int, matrix bool) (numRows, numColumns int) {
	if matrix {
		utils.IncreaseToNextSquare(&numBlocks)
		numColumns = int(math.Sqrt(float64(numBlocks)))
		numRows = numColumns
	} else {
		numColumns = numBlocks
		numRows = 1
	}
	return
}

/*
func (d *DB) SetEntry(i int, el uint32) {
	d.Entries[i] = []byte(el)
}

func (d *DB) AppendBlock(bl []uint32) {
	d.Entries = append(d.Entries, bl...)
}

func (d *DB) GetEntry(i int) uint32 {
	return d.Entries[i]
}

*/
func (d *DB) Range(begin, end int) []uint32 {
	return d.Entries[begin:end]
}

/*
func InitMultiBitDBWithCapacity(numRows, numColumns, blockSize, cap int) (*DB, error) {
	info := Info{NumColumns: numColumns,
		NumRows:   numRows,
		BlockSize: blockSize,
	}

	db, err := NewEmptyDBWithCapacity(info, cap)
	if err != nil {
		return nil, xerrors.Errorf("failed to create db: %v", err)
	}

	db.BlockLengths = make([]int, numRows*numColumns)

	return db, nil
}

/*
func InitMultiBitDB(numRows, numColumns, blockSize int) (*DB, error) {
	info := Info{NumColumns: numColumns,
		NumRows:   numRows,
		BlockSize: blockSize,
	}

	db, err := NewEmptyDB(info)
	if err != nil {
		return nil, xerrors.Errorf("failed to create db: %v", err)
	}

	return db, nil
}

func CreateRandomMultiBitDB(rnd io.Reader, dbLen, numRows, blockLen int) (*DB, error) {
	numColumns := dbLen / (8 * field.Bytes * numRows * blockLen)
	// handle very small db
	if numColumns == 0 {
		numColumns = 1
	}

	info := Info{
		NumColumns: numColumns,
		NumRows:    numRows,
		BlockSize:  blockLen,
	}

	n := numRows * numColumns * blockLen

	bytesLength := n*field.Bytes + 1
	bytes := make([]byte, bytesLength)

	if _, err := io.ReadFull(rnd, bytes[:]); err != nil {
		return nil, xerrors.Errorf("failed to read random bytes: %v", err)
	}

	db, err := NewDB(info)
	if err != nil {
		return nil, xerrors.Errorf("failed to create db: %v", err)
	}

	// add block lengths also in this case for compatibility
	db.BlockLengths = make([]int, numRows*numColumns)

	for i := 0; i < n; i++ {
		element := binary.BigEndian.Uint32(bytes[i*field.Bytes:(i+1)*field.Bytes]) % field.ModP
		db.SetEntry(i, element)
		db.BlockLengths[i/blockLen] = blockLen
	}

	return db, nil
}
*/
