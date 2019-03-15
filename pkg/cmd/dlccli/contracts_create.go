package dlccli

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
	"github.com/p2pderivatives/dlc/internal/dlcmgr"
	"github.com/p2pderivatives/dlc/pkg/dlc"
	"github.com/p2pderivatives/dlc/pkg/oracle"
	"github.com/p2pderivatives/dlc/pkg/utils"
	"github.com/p2pderivatives/dlc/pkg/wallet"
	"github.com/spf13/cobra"
)

var fund1 int
var fund2 int
var address1 string
var address2 string
var changeAddress1 string
var changeAddress2 string
var fundtxFeerate int
var redeemtxFeerate int
var refundlc int
var dealsFile string
var opubfile string
var wallet1 string
var wallet2 string
var pubpass1 string
var pubpass2 string
var privpass1 string
var privpass2 string

// Contractor is contractor
type Contractor struct {
	wallet   wallet.Wallet
	builder  *dlc.Builder
	manager  *dlcmgr.Manager
	pubpass  string
	privpass string
}

func runCreateContract(cmd *cobra.Command, args []string) {
	var err error
	party1 := initFirstParty()
	party2 := initSecondParty()
	pubset := parseOraclePubkey()

	// Both set oracle's pubkey
	fmt.Printf("Setting oracle's pubkey\n")
	party1.builder.SetOraclePubkeySet(pubset)
	party2.builder.SetOraclePubkeySet(pubset)

	fmt.Println("First party preparing public key and utxos")

	// FirstParty prepares pubkeys
	err = party1.builder.PreparePubkey()
	errorHandler(err)

	// FirstParty prepares utxos
	err = party1.builder.PrepareFundTx()
	errorHandler(err)

	fmt.Println("First party sending public key and utxos to second party")

	// First Party sends offer to Second Party
	p1, err := party1.builder.PublicKey()
	errorHandler(err)
	u1 := party1.builder.Utxos()
	chaddr1 := party1.builder.ChangeAddress()

	fmt.Println("Second party accepting public key, utxos and change address")

	// Second party accepts pubkey, utxos, addresses
	err = party2.builder.AcceptPubkey(p1)
	errorHandler(err)
	err = party2.builder.AcceptUtxos(u1)
	errorHandler(err)
	party2.builder.AcceptsChangeAdderss(chaddr1)

	fmt.Println("Second party preparing public key and utxos")

	// Second Party signs CETxs and RefundTx
	err = party2.builder.PreparePubkey()
	errorHandler(err)
	err = party2.builder.PrepareFundTx()
	errorHandler(err)

	fmt.Println("Second party sigining CETxs and RefundTx")

	ceSigs2, err := party2.builder.SignContractExecutionTxs()
	errorHandler(err)
	refundSig2, err := party2.builder.SignRefundTx()
	errorHandler(err)

	fmt.Println("Second party sending public key, utxos and change address")
	p2, err := party2.builder.PublicKey()
	errorHandler(err)
	u2 := party2.builder.Utxos()
	chaddr2 := party2.builder.ChangeAddress()

	fmt.Println("First party accepting public key, utxoa and change address")

	err = party1.builder.AcceptPubkey(p2)
	errorHandler(err)
	err = party1.builder.AcceptUtxos(u2)
	errorHandler(err)
	party1.builder.AcceptsChangeAdderss(chaddr2)
	errorHandler(err)

	fmt.Println("First party accepting signatures of CETXs and RefundTx")

	// FirstParty accepts sigs
	err = party1.builder.AcceptRefundTxSignature(refundSig2)
	errorHandler(err)
	err = party1.builder.AcceptCETxSignatures(ceSigs2)
	errorHandler(err)

	fmt.Println("First party sigining all transactions")

	// FirstParty signs CETxs and RefundTx and FundTx
	ceSigs1, err := party1.builder.SignContractExecutionTxs()
	errorHandler(err)
	refundSig1, err := party1.builder.SignRefundTx()
	errorHandler(err)
	fundWits1, err := party1.builder.SignFundTx()
	errorHandler(err)

	fmt.Println("Second party accepting all signatures")

	// SecondParty accepts sigs
	err = party2.builder.AcceptCETxSignatures(ceSigs1)
	errorHandler(err)
	err = party2.builder.AcceptRefundTxSignature(refundSig1)
	errorHandler(err)
	party2.builder.AcceptFundWitnesses(fundWits1)

	// SecondParty sends FundTx signature
	fundWits2, err := party2.builder.SignFundTx()
	errorHandler(err)
	party1.builder.AcceptFundWitnesses(fundWits2)

	fmt.Println("First party persisting contract")

	d1 := party1.builder.Contract
	ID1, err := d1.ContractID()
	errorHandler(err)
	key1, err := chainhash.NewHashFromStr(ID1)
	errorHandler(err)
	err = party1.manager.StoreContract(key1.CloneBytes(), d1)
	errorHandler(err)

	fmt.Println("Second party persisting contract")

	d2 := party2.builder.Contract
	ID2, err := d2.ContractID()
	errorHandler(err)
	key2, err := chainhash.NewHashFromStr(ID2)
	errorHandler(err)
	err = party2.manager.StoreContract(key2.CloneBytes(), d2)
	errorHandler(err)

	if ID1 != ID2 {
		err = fmt.Errorf("contract IDs must be same, but different")
		errorHandler(err)
	}

	fmt.Println("Second party constructing FundTx")

	// SecondParty create FundTx
	fundtx, err := party2.builder.Contract.FundTx()
	errorHandler(err)
	fundtxHex, err := utils.TxToHex(fundtx)
	errorHandler(err)
	refundtx, err := party2.builder.Contract.SignedRefundTx()
	errorHandler(err)
	refundtxHex, err := utils.TxToHex(refundtx)
	errorHandler(err)

	fmt.Println("Contract created")
	fmt.Printf("\nContractID: \n%s\n", ID1)
	fmt.Printf("\nFundTx hex:\n%s\n", fundtxHex)
	fmt.Printf("\nRefundTx hex:\n%s\n", refundtxHex)
}

func initCreateContractCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "create",
		Short: "Create contract",
		Run:   runCreateContract,
	}

	cmd.Flags().StringVar(&fixingTime, "fixingtime", "", "Fixing time")
	cmd.MarkFlagRequired("fixingtime")
	cmd.Flags().IntVar(&fund1, "fund1", 0, "Fund amount of First party (satoshi)")
	cmd.MarkFlagRequired("fund1")
	cmd.Flags().IntVar(&fund2, "fund2", 0, "Fund amount of Second party (satoshi)")
	cmd.MarkFlagRequired("fund2")
	cmd.Flags().StringVar(&address1, "address1", "", "Transfer address of First party")
	cmd.MarkFlagRequired("address1")
	cmd.Flags().StringVar(&address2, "address2", "", "Transfer address of Second party")
	cmd.MarkFlagRequired("address2")
	cmd.Flags().StringVar(&changeAddress1, "change_address1", "", "Change address of First party")
	cmd.Flags().StringVar(&changeAddress2, "change_address2", "", "Change address of Second party")
	cmd.Flags().IntVar(&fundtxFeerate, "fundtx_feerate", 0, "Fee rate for fund tx (satoshi/byte)")
	cmd.MarkFlagRequired("fundtx_feerate")
	cmd.Flags().IntVar(&redeemtxFeerate, "redeemtx_feerate", 0, "Fee rate for refund tx, cetx, closing tx (satoshi/byte)")
	cmd.MarkFlagRequired("redeemtx_feerate")
	cmd.Flags().IntVar(&refundlc, "refund_locktime", 0, "Locktime of refune tx (block height)")
	cmd.MarkFlagRequired("refund_locktime")
	cmd.Flags().StringVar(&dealsFile, "deals_file", "", "Path to a csv file that contains deals")
	cmd.MarkFlagRequired("deals_file")
	cmd.Flags().StringVar(&opubfile, "oracle_pubkey", "", "Oracle's pubkey json file")
	cmd.MarkFlagRequired("oracle_pubkey")
	cmd.Flags().StringVar(&walletDir, "walletdir", "", "Wallet directory")
	cmd.MarkFlagRequired("walletdir")
	cmd.Flags().StringVar(&wallet1, "wallet1", "", "Wallet name of First Party")
	cmd.MarkFlagRequired("wallet1")
	cmd.Flags().StringVar(&wallet2, "wallet2", "", "Wallet name of Second Party")
	cmd.MarkFlagRequired("wallet_2")
	cmd.Flags().StringVar(&pubpass1, "pubpass1", "", "Pubpass phrase of First party's wallet")
	cmd.MarkFlagRequired("pubpass1")
	cmd.Flags().StringVar(&pubpass2, "pubpass2", "", "Pubpass phrase of Second party's wallet")
	cmd.MarkFlagRequired("pubpass2")
	cmd.Flags().StringVar(&privpass1, "privpass1", "", "Privpass phrase of First party's wallet")
	cmd.MarkFlagRequired("privpass1")
	cmd.Flags().StringVar(&privpass2, "privpass2", "", "Privpass phrase of Second party's wallet")
	cmd.MarkFlagRequired("privpass2")

	return cmd
}

func loadDeals() []*dlc.Deal {
	f, err := os.Open(dealsFile)
	errorHandler(err)

	// TOOD: give nDigits from outside
	nDigits := 5

	deals := []*dlc.Deal{}
	r := csv.NewReader(bufio.NewReader(f))
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		errorHandler(err)

		deal := convertRowToDeal(row, nDigits)
		deals = append(deals, deal)
	}

	return deals
}

func convertRowToDeal(rec []string, nDigits int) *dlc.Deal {
	v, err := strconv.Atoi(rec[0])
	errorHandler(err)

	msgs := oracle.NumberToByteMsgs(v, nDigits)

	amt1, err := strconv.Atoi(rec[1])
	errorHandler(err)
	amt2, err := strconv.Atoi(rec[2])
	errorHandler(err)

	deal := dlc.NewDeal(
		btcutil.Amount(amt1),
		btcutil.Amount(amt2),
		msgs)

	return deal
}

func initFirstParty() *Contractor {
	w, wdb := openWallet(pubpass1, walletDir, wallet1)
	err := w.Unlock([]byte(privpass1))
	errorHandler(err)
	mgr, err := dlcmgr.Open(wdb)
	errorHandler(err)
	conds := loadDLCConditions()
	d := dlc.NewDLC(conds)
	b := dlc.NewBuilder(dlc.FirstParty, w, d)

	return &Contractor{
		wallet:   w,
		builder:  b,
		manager:  mgr,
		pubpass:  pubpass1,
		privpass: privpass1,
	}
}

func initSecondParty() *Contractor {
	w, wdb := openWallet(pubpass2, walletDir, wallet2)
	err := w.Unlock([]byte(privpass2))
	errorHandler(err)
	mgr, err := dlcmgr.Open(wdb)
	errorHandler(err)
	conds := loadDLCConditions()
	d := dlc.NewDLC(conds)
	b := dlc.NewBuilder(dlc.SecondParty, w, d)

	return &Contractor{
		wallet:   w,
		builder:  b,
		manager:  mgr,
		pubpass:  pubpass2,
		privpass: privpass2,
	}
}

func parseOraclePubkey() *oracle.PubkeySet {
	data, err := ioutil.ReadFile(opubfile)
	errorHandler(err)

	pubset := &oracle.PubkeySet{}
	json.Unmarshal(data, pubset)

	return pubset
}

func loadDLCConditions() *dlc.Conditions {
	ftime := parseFixingTimeFlag()

	// cast int to btcutil.Amount
	famt1 := btcutil.Amount(fund1)
	famt2 := btcutil.Amount(fund2)
	ffrate := btcutil.Amount(fundtxFeerate)
	rfrate := btcutil.Amount(redeemtxFeerate)

	// TODO: confirm how to convert timestamp to locktime
	lc := uint32(refundlc)

	deals := loadDeals()

	net := loadChainParams(bitcoinConf)
	conds, err := dlc.NewConditions(
		net, ftime, famt1, famt2, ffrate, rfrate, lc, deals)
	errorHandler(err)

	return conds
}