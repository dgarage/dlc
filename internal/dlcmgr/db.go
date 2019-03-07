package dlcmgr

import (
	"errors"
	"fmt"

	"github.com/btcsuite/btcwallet/walletdb"
)

var (
	nsTop        = []byte("dlcmgr")
	nsContracts  = []byte("contracts")
	nsConditions = []byte("conds")
	nsPubkeys    = []byte("pubkeys")
)

func createManager(db walletdb.DB) error {
	err := walletdb.Update(db, func(tx walletdb.ReadWriteTx) error {
		ns, e := tx.CreateTopLevelBucket(nsTop)
		if e != nil {
			return e
		}

		_, e = ns.CreateBucket(nsContracts)
		return e
	})

	return err
}

func openManager(db walletdb.DB) *Manager {
	mgr := &Manager{db: db}
	return mgr
}

func (m *Manager) updateContractBucket(
	k []byte, f func(walletdb.ReadWriteBucket) error) error {
	updateFunc := func(tx walletdb.ReadWriteTx) error {
		ns := tx.ReadWriteBucket(nsTop)
		contractsNS := ns.NestedReadWriteBucket(nsContracts)
		bucket, e := contractsNS.CreateBucketIfNotExists(k)
		if e != nil {
			return e
		}
		return f(bucket)
	}
	return walletdb.Update(m.db, updateFunc)
}

// ContractNotExistsError gets raised when contract doesn't exist
type ContractNotExistsError struct {
	error
}

func newContractNotExistsError(
	k []byte) *ContractNotExistsError {
	msg := fmt.Sprintf("Contract not exists. key: %s", k)
	return &ContractNotExistsError{error: errors.New(msg)}
}

func (m *Manager) viewContractBucket(
	k []byte, f func(walletdb.ReadBucket) error) error {
	viewFunc := func(tx walletdb.ReadTx) error {
		ns := tx.ReadBucket(nsTop)
		contractsNS := ns.NestedReadBucket(nsContracts)
		bucket := contractsNS.NestedReadBucket(k)
		if bucket == nil {
			return newContractNotExistsError(k)
		}
		return f(bucket)
	}
	return walletdb.View(m.db, viewFunc)
}