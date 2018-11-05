package wallet

import (
	"encoding/hex"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcwallet/waddrmgr"
	"github.com/btcsuite/btcwallet/walletdb"
	"github.com/btcsuite/btcwallet/wtxmgr"
)

// Utxo is a unspend transaction output
type Utxo = btcjson.ListUnspentResult

// ListUnspent returns unspent transactions.
// TODO: add filter
//   Only utxos with address contained the param addresses will be considered.
//   If param addresses is empty, all addresses are considered and there is no
//   filter
func (w *wallet) ListUnspent() (utxos []*Utxo, err error) {
	var results []*btcjson.ListUnspentResult
	err = walletdb.View(w.db, func(tx walletdb.ReadTx) error {
		addrmgrNs := tx.ReadBucket(waddrmgrNamespaceKey)
		txmgrNs := tx.ReadBucket(wtxmgrNamespaceKey)

		syncBlock := w.manager.SyncedTo()
		// filter := len(addresses) != 0

		unspent, e := w.txStore.UnspentOutputs(txmgrNs)
		if e != nil {
			return e
		}

		// utxos = make([]*btcjson.ListUnspentResult, 0, len(unspent))
		for i := range unspent {
			output := unspent[i]
			result := w.credit2ListUnspentResult(output, syncBlock, addrmgrNs)
			// TODO: result might return nil... catch that nil?
			results = append(results, result)
		}
		return nil
	})
	utxos = results
	return utxos, err
}

func (w *wallet) credit2ListUnspentResult(
	c wtxmgr.Credit,
	syncBlock waddrmgr.BlockStamp,
	addrmgrNs walletdb.ReadBucket) *btcjson.ListUnspentResult {

	// TODO: add minconf, maxconf params
	confs := confirms(c.Height, syncBlock.Height)
	// // Outputs with fewer confirmations than the minimum or more
	// // confs than the maximum are excluded.
	// confs := confirms(output.Height, syncBlock.Height)
	// if confs < minconf || confs > maxconf {
	// 	continue
	// }

	// Only mature coinbase outputs are included.
	if c.FromCoinBase {
		target := int32(w.params.CoinbaseMaturity) // make param
		if !confirmed(target, c.Height, syncBlock.Height) {
			// continue
			return nil // maybe?

		}
	}

	// TODO: exclude locked outputs from result set.
	// Exclude locked outputs from the result set.

	// Lookup the associated account for the output.  Use the
	// default account name in case there is no associated account
	// for some reason, although this should never happen.
	//
	// This will be unnecessary once transactions and outputs are
	// grouped under the associated account in the db.
	defaultAccountName := "default"
	acctName := defaultAccountName
	sc, addrs, _, err := txscript.ExtractPkScriptAddrs(
		c.PkScript, w.params)
	if err != nil {
		// continue
		return nil // maybe?
	}
	if len(addrs) > 0 {
		smgr, acct, err := w.manager.AddrAccount(addrmgrNs, addrs[0])
		if err == nil {
			s, err := smgr.AccountName(addrmgrNs, acct)
			if err == nil {
				acctName = s
			}
		}
	}

	// not including this part bc this func will assume there is no filter
	// 	if filter {
	// 		for _, addr := range addrs {
	// 			_, ok := addresses[addr.EncodeAddress()]
	// 			if ok {
	// 				goto include
	// 			}
	// 		}
	// 		// continue
	// 		return nil // maybe?
	// 	}
	// include:

	result := &btcjson.ListUnspentResult{
		TxID:          c.OutPoint.Hash.String(),
		Vout:          c.OutPoint.Index,
		Account:       acctName,
		ScriptPubKey:  hex.EncodeToString(c.PkScript),
		Amount:        c.Amount.ToBTC(),
		Confirmations: int64(confs),
		Spendable:     w.isSpendable(sc, addrs, addrmgrNs),
	}

	// BUG: this should be a JSON array so that all
	// addresses can be included, or removed (and the
	// caller extracts addresses from the pkScript).
	if len(addrs) > 0 {
		result.Address = addrs[0].EncodeAddress()
	}

	return result
}

// isSpendable determines if given ScriptClass is spendable or not.
// Does NOT support watch-only addresses. This func will need to be rewritten
// to support watch-only addresses
func (w *wallet) isSpendable(sc txscript.ScriptClass, addrs []btcutil.Address,
	addrmgrNs walletdb.ReadBucket) (spendable bool) {
	// At the moment watch-only addresses are not supported, so all
	// recorded outputs that are not multisig are "spendable".
	// Multisig outputs are only "spendable" if all keys are
	// controlled by this wallet.
	//
	// TODO: Each case will need updates when watch-only addrs
	// is added.  For P2PK, P2PKH, and P2SH, the address must be
	// looked up and not be watching-only.  For multisig, all
	// pubkeys must belong to the manager with the associated
	// private key (currently it only checks whether the pubkey
	// exists, since the private key is required at the moment).
scSwitch:
	switch sc {
	case txscript.PubKeyHashTy:
		spendable = true
	case txscript.PubKeyTy:
		spendable = true
	case txscript.WitnessV0ScriptHashTy:
		spendable = true
	case txscript.WitnessV0PubKeyHashTy:
		spendable = true
	case txscript.MultiSigTy:
		for _, a := range addrs {
			_, err := w.manager.Address(addrmgrNs, a)
			if err == nil {
				continue
			}
			if waddrmgr.IsError(err, waddrmgr.ErrAddressNotFound) {
				break scSwitch
			}
			// return err TODO: figure out what to replace the return error
		}
		spendable = true
	}

	return spendable
}

// confirms returns the number of confirmations for a transaction in a block at
// height txHeight (or -1 for an unconfirmed tx) given the chain height
// curHeight.
func confirms(txHeight, curHeight int32) int32 {
	switch {
	case txHeight == -1, txHeight > curHeight:
		return 0
	default:
		return curHeight - txHeight + 1
	}
}

// confirmed checks whether a transaction at height txHeight has met minconf
// confirmations for a blockchain at height curHeight.
func confirmed(minconf, txHeight, curHeight int32) bool {
	return confirms(txHeight, curHeight) >= minconf
}
