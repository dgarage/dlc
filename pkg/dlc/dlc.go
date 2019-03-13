package dlc

import (
	"errors"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/p2pderivatives/dlc/pkg/script"
	"github.com/p2pderivatives/dlc/pkg/wallet"
	validator "gopkg.in/go-playground/validator.v9"
)

// DLC contains all information required for DLC contract
// including FundTx, SettlementTx, RefundTx
type DLC struct {
	NetParams   *chaincfg.Params
	Conds       *Conditions
	Oracle      *Oracle
	Pubs        map[Contractor]*btcec.PublicKey // pubkeys used for script and txout
	Addrs       map[Contractor]btcutil.Address  // addresses used to distribute funds after fixing deal
	ChangeAddrs map[Contractor]btcutil.Address  // addresses used to send change
	Utxos       map[Contractor][]*Utxo
	FundWits    map[Contractor][]wire.TxWitness // TODO: change to fund signatures
	RefundSigs  map[Contractor][]byte           // signatures for refund tx
	ExecSigs    [][]byte                        // counterparty's signatures for CETxs
}

// Utxo is alias of btcjson.ListUnspentResult
type Utxo = btcjson.ListUnspentResult

// NewDLC initializes DLC
func NewDLC(conds *Conditions, net *chaincfg.Params) *DLC {
	nDeal := len(conds.Deals)
	return &DLC{
		NetParams:   net,
		Conds:       conds,
		Oracle:      NewOracle(nDeal),
		Pubs:        make(map[Contractor]*btcec.PublicKey),
		Addrs:       make(map[Contractor]btcutil.Address),
		ChangeAddrs: make(map[Contractor]btcutil.Address),
		Utxos:       make(map[Contractor][]*Utxo),
		FundWits:    make(map[Contractor][]wire.TxWitness),
		RefundSigs:  make(map[Contractor][]byte),
		ExecSigs:    make([][]byte, nDeal),
	}
}

// Conditions contains conditions of a contract
type Conditions struct {
	FixingTime     time.Time                     `validate:"required,gt=time.Now()"`
	FundAmts       map[Contractor]btcutil.Amount `validate:"required,dive,gt=0"`
	FundFeerate    btcutil.Amount                `validate:"required,gt=0"` // fund fee rate (satoshi per byte)
	RedeemFeerate  btcutil.Amount                `validate:"required,gt=0"` // redeem fee rate (satoshi per byte)
	RefundLockTime uint32                        `validate:"required,gt=0"` // refund locktime (block height)
	Deals          []*Deal                       `validate:"required,gt=0,dive,required"`
}

// NewConditions creates a new DLC conditions
func NewConditions(
	ftime time.Time,
	famt1, famt2 btcutil.Amount,
	ffeerate, rfeerate btcutil.Amount, // fund feerate and redeem feerate
	refundLockTime uint32, // refund locktime
	deals []*Deal,
) (*Conditions, error) {
	famts := make(map[Contractor]btcutil.Amount)
	famts[FirstParty] = famt1
	famts[SecondParty] = famt2

	conds := &Conditions{
		FixingTime:     ftime,
		FundAmts:       famts,
		FundFeerate:    ffeerate,
		RedeemFeerate:  rfeerate,
		RefundLockTime: refundLockTime,
		Deals:          deals,
	}

	// validate structure
	err := validator.New().Struct(conds)

	return conds, err
}

// ClosingTxOut returns a final txout owned only by a given party
func (d *DLC) ClosingTxOut(
	p Contractor, amt btcutil.Amount) (*wire.TxOut, error) {
	pub := d.Pubs[p]
	if pub == nil {
		return nil, errors.New("missing pubkey")
	}

	pkScript, err := script.P2WPKHpkScript(pub)
	if err != nil {
		return nil, err
	}

	txout := wire.NewTxOut(int64(amt), pkScript)
	return txout, nil
}

const txVersion = 2

// Contractor represents a contractor type
type Contractor int

const (
	// FirstParty is a contractor who creates offer
	FirstParty Contractor = 0
	// SecondParty is a contractor who accepts offer
	SecondParty Contractor = 1
)

// String represents contractor in string format
func (p Contractor) String() string {
	switch p {
	case FirstParty:
		return "first party"
	case SecondParty:
		return "second party"
	}
	return ""
}

// counterparty returns the counterparty
func counterparty(p Contractor) (cp Contractor) {
	switch p {
	case FirstParty:
		cp = SecondParty
	case SecondParty:
		cp = FirstParty
	}
	return cp
}

// Builder builds DLC by interacting with wallet
type Builder struct {
	party  Contractor
	wallet wallet.Wallet
	dlc    *DLC
}

// NewBuilder creates a new Builder for a contractor
func NewBuilder(
	p Contractor, w wallet.Wallet, conds *Conditions, net *chaincfg.Params,
) *Builder {
	return &Builder{
		dlc:    NewDLC(conds, net),
		party:  p,
		wallet: w,
	}
}

// DLC returns the DLC constructed by builder
func (b *Builder) DLC() *DLC {
	return b.dlc
}

// PreparePubkey sets fund pubkey
func (b *Builder) PreparePubkey() error {
	pub, err := b.wallet.NewPubkey()
	if err != nil {
		return err
	}
	b.dlc.Pubs[b.party] = pub
	return nil
}

// AcceptPubkey accepts counter party's public key
func (b *Builder) AcceptPubkey(pub []byte) error {
	p, err := btcec.ParsePubKey(pub, btcec.S256())
	c := counterparty(b.party)

	b.dlc.Pubs[c] = p

	return err
}

// PubkeyNotExistsError is error when either public key doesn't exist
type PubkeyNotExistsError struct{ error }

// PublicKey returns serialized public key (compressed)
func (b *Builder) PublicKey() ([]byte, error) {
	pub, ok := b.dlc.Pubs[b.party]
	if !ok {
		return nil, &PubkeyNotExistsError{}
	}
	return pub.SerializeCompressed(), nil
}
