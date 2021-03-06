package wallet

import (
	"testing"

	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/p2pderivatives/dlc/internal/mocks/rpcmock"
	"github.com/p2pderivatives/dlc/internal/test"
	"github.com/p2pderivatives/dlc/pkg/script"
	"github.com/stretchr/testify/assert"
)

func TestWitnessSignature(t *testing.T) {
	assert := assert.New(t)

	w, tearDownFunc := setupWallet(t)
	defer tearDownFunc()

	rpcc := &rpcmock.Client{}
	rpcc = mockImportAddress(rpcc, nil)
	w.rpc = rpcc

	// pubkey and pk script
	pub, _ := w.NewPubkey()
	pkScript, _ := script.P2WPKHpkScript(pub)

	// prepare source tx
	amt := btcutil.Amount(10000)
	sourceTx := test.NewSourceTx()
	sourceTx.AddTxOut(wire.NewTxOut(int64(amt), pkScript))

	redeemTx := test.NewRedeemTx(sourceTx, 0)

	// should fail if it's not unlocked
	_, err := w.WitnessSignature(redeemTx, 0, amt, pkScript, pub)
	assert.NotNil(err)

	// unlock for private key access
	w.Unlock(testPrivPass)

	sign, err := w.WitnessSignature(redeemTx, 0, amt, pkScript, pub)
	assert.Nil(err)

	wt := wire.TxWitness{sign, pub.SerializeCompressed()}
	redeemTx.TxIn[0].Witness = wt

	// execute script
	err = test.ExecuteScript(pkScript, redeemTx, int64(amt))
	assert.Nil(err)
}
