package dlc

import (
	"errors"

	"github.com/btcsuite/btcd/btcec"
	"github.com/dgarage/dlc/internal/oracle"
	"github.com/dgarage/dlc/internal/schnorr"
)

// OracleRequirements contains pubkeys and commitments and sign received from oracle
type OracleRequirements struct {
	pubkeySet   *oracle.PubkeySet  // Oracle's pubkey set
	commitments []*btcec.PublicKey // Commitments for deals
	sign        []byte             // Sign for a fixed deal
	signedMsgs  [][]byte           // Messages signed by Oracle
}

func newOracleReqs(n int) *OracleRequirements {
	return &OracleRequirements{
		commitments: make([]*btcec.PublicKey, n)}
}

// PrepareOracleCommitments prepares oracle's commitments for all deals
func (d *DLC) PrepareOracleCommitments(
	V *btcec.PublicKey, Rs []*btcec.PublicKey) {
	Cs := make([]*btcec.PublicKey, len(d.conds.Deals))

	for i, deal := range d.conds.Deals {
		Cs[i] = schnorr.CommitMulti(V, Rs, deal.Msgs)
	}

	d.oracleReqs.commitments = Cs
}

// SetOraclePubkeySet sets oracle's pubkey set
func (b *Builder) SetOraclePubkeySet(pubset *oracle.PubkeySet) {
	b.dlc.PrepareOracleCommitments(
		pubset.Pubkey, pubset.CommittedRpoints)
	b.dlc.oracleReqs.pubkeySet = pubset
}

// FixDeal fixes a deal by setting the signature provided by oracle
func (d *DLC) FixDeal(msgs [][]byte, signs [][]byte) error {
	dID, _, err := d.DealByMsgs(msgs)
	if err != nil {
		return err
	}

	C := d.oracleReqs.commitments[dID]
	s := schnorr.SumSigns(signs)

	ok := schnorr.Verify(C, s)
	if !ok {
		return errors.New("invalid oracle sign")
	}

	// set fixed messages and sign for it
	d.oracleReqs.signedMsgs = msgs
	d.oracleReqs.sign = s

	return nil
}

// FixDeal fixes a deal by a oracle's sign set
func (b *Builder) FixDeal(signSet *oracle.SignSet) error {
	return b.dlc.FixDeal(signSet.Msgs, signSet.Signs)
}

// FixedDeal returns a fixed deal
func (d *DLC) FixedDeal() (idx int, deal *Deal, err error) {
	if !d.HasDealFixed() {
		err = newNoFixedDealError()
		return
	}
	return d.DealByMsgs(d.oracleReqs.signedMsgs)
}

// HasDealFixed checks if a deal has been fixed
func (d *DLC) HasDealFixed() bool {
	return d.oracleReqs.signedMsgs != nil && d.oracleReqs.sign != nil
}
