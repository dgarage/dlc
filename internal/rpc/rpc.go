// Package rpc project rpc.go
package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

// Client is an interface that provides access to certain methods of type rpcclient.Client
type Client interface {
	ListUnspentMinMaxAddresses(
		minConf, maxConf int, addrs []btcutil.Address,
	) ([]btcjson.ListUnspentResult, error)
	ImportAddressRescan(address string, account string, rescan bool) error
	SendRawTransaction(tx *wire.MsgTx, allowHighFees bool) (*chainhash.Hash, error)
	SendToAddress(address btcutil.Address, amount btcutil.Amount) (*chainhash.Hash, error)
	Generate(numBlocks uint32) ([]*chainhash.Hash, error)
	GetBlockCount() (int64, error)
	RawRequest(method string, params []json.RawMessage) (json.RawMessage, error)
	// TODO: add Shutdown func
}

const (
	defaultHost         = "localhost"
	defaultHTTPPostMode = true
	defaultDisableTLS   = true
)

// NewClient returns Client interface object
func NewClient(cfgPath string) (Client, error) {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return nil, err
	}
	return newClient(cfg)
}

func newClient(cfg *rpcclient.ConnConfig) (*rpcclient.Client, error) {
	return rpcclient.New(cfg, nil)
}

func loadConfig(cfgPath string) (*rpcclient.ConnConfig, error) {
	cfgFile, err := os.Open(cfgPath)
	if err != nil {
		return nil, err
	}
	defer cfgFile.Close()

	content, err := ioutil.ReadAll(cfgFile)
	if err != nil {
		return nil, err
	}

	// Extract the rpcuser
	rpcUserRegexp, err := regexp.Compile(`(?m)^\s*rpcuser=([^\s]+)`)
	if err != nil {
		return nil, err
	}
	userSubmatches := rpcUserRegexp.FindSubmatch(content)
	if userSubmatches == nil {
		return nil, errors.New("rpcuser isn't set in config file")
	}
	user := strings.Split(string(userSubmatches[0]), "=")[1]

	// Extract the rpcpassword
	rpcPassRegexp, err := regexp.Compile(`(?m)^\s*rpcpassword=([^\s]+)`)
	if err != nil {
		return nil, err
	}
	passSubmatches := rpcPassRegexp.FindSubmatch(content)
	if passSubmatches == nil {
		return nil, errors.New("rpcpassword isn't set in config file")
	}
	pass := strings.Split(string(passSubmatches[0]), "=")[1]

	// Extract the regtest
	port, err := detectRPCPort(content)
	if err != nil {
		return nil, err
	}

	cfg := &rpcclient.ConnConfig{
		Host:         net.JoinHostPort(defaultHost, port),
		User:         user,
		Pass:         pass,
		HTTPPostMode: defaultHTTPPostMode, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   defaultDisableTLS,   // Bitcoin core does not provide TLS by default
	}
	return cfg, nil
}

func detectRPCPort(content []byte) (string, error) {
	use, err := useTestnet(content)
	if err != nil {
		return "", err
	}
	if use {
		return "18332", nil
	}

	use, err = useRegtest(content)
	if err != nil {
		return "", err
	}
	if use {
		return "18443", nil
	}

	return "8332", nil
}
func useTestnet(content []byte) (bool, error) {
	return checkNetFlag("testnet", content)
}

func useRegtest(content []byte) (bool, error) {
	return checkNetFlag("regtest", content)
}

func checkNetFlag(net string, content []byte) (bool, error) {
	re, err := regexp.Compile(fmt.Sprintf(`(?m)^\s*%s=([^\s]+)`, net))
	if err != nil {
		return false, err
	}
	matches := re.FindSubmatch(content)
	if matches == nil {
		return false, nil
	}
	v := strings.Split(string(matches[0]), "=")[1]
	return v == "1", nil
}
