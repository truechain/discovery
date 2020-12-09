package token

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math"
	"math/big"
	"os"
	"strings"
	"time"
	"truechain/discovery"
	"truechain/discovery/accounts/abi"
	"truechain/discovery/accounts/abi/bind"
	"truechain/discovery/accounts/abi/bind/backends"
	"truechain/discovery/common"
	"truechain/discovery/contracts/uniswap/TokenE"
	tokenf "truechain/discovery/contracts/uniswap/TokenF"
	"truechain/discovery/contracts/uniswap/tokenm"
	"truechain/discovery/core/types"
	"truechain/discovery/crypto"
	"truechain/discovery/etrueclient"
	"truechain/discovery/log"
)

var (
	key, _ = crypto.HexToECDSA("142778b7a14777c8d3d0f5d7ed913ab87dd0ca8fd008c2e76567c5761424c013")
	//key, _ = crypto.HexToECDSA("c1581e25937d9ab91421a3e1a2667c85b0397c75a195e643109938e987acecfc")
	addr = crypto.PubkeyToAddress(key.PublicKey)

	key2, _      = crypto.HexToECDSA("5e6ea3e3ba8a3d8940088247eda01a0909320f729ae3afcdc5747b2ced1ac460")
	testAddr     = crypto.PubkeyToAddress(key2.PublicKey)
	nonce        uint64
	routerAbi, _ = abi.JSON(strings.NewReader(TokenABI))
	mapAbi, _    = abi.JSON(strings.NewReader(tokenm.TokenmABI))
)

func packInput(routerStaking abi.ABI, abiMethod, method string, params ...interface{}) []byte {
	input, err := routerStaking.Pack(abiMethod, params...)
	if err != nil {
		printTest(method, " error ", err)
	}
	return input
}

func printTest(a ...interface{}) {
	log.Info("test", "SendTX", a)
}

func simulateRouter(transactOpts *bind.TransactOpts, contractBackend *backends.SimulatedBackend, basecontract *BaseContract, routercontract *RouterContract) {
	tik := new(big.Int).SetUint64(10000000000000000)
	tik1 := new(big.Int).SetUint64(1000000000000)
	balance, err := basecontract.mapTran.Allowance(nil, addr, routercontract.routerAddr)
	name, err := basecontract.mapTran.Name(nil)
	fmt.Println("balance ", balance, "name", name, " routerAddr ", routercontract.routerAddr.String(), " mapT ", basecontract.mapT.String(), "err", err)
	transactOpts.Value = new(big.Int).SetUint64(1000000000000000000)
	_, err = routercontract.RTran.AddLiquidityETH(transactOpts, basecontract.mapT, tik, tik, tik1, addr, new(big.Int).SetUint64(1699658290))
	fmt.Println("simulate result", err, " routerAddr ", routercontract.rethR.String(), " addr ", addr.String())
	contractBackend.Commit()
}

func sendRouterContract(transactOpts *bind.TransactOpts, contractBackend *backends.SimulatedBackend, client *etrueclient.Client, basecontract *BaseContract) (*RouterContract, bool) {
	routerAddr, _, _, err := DeployToken(transactOpts, contractBackend, basecontract.fac, basecontract.weth)
	RTran, err := NewToken(routerAddr, contractBackend)

	_, err = basecontract.mapTran.Approve(transactOpts, routerAddr, new(big.Int).SetUint64(1000000000000000000))
	contractBackend.Commit()
	if err != nil {
		fmt.Println("sendRouterContract", err)
	}
	_, rtx, _, err := DeployToken(transactOpts, contractBackend, basecontract.facR, basecontract.wethR)

	rHash := sendContractTransaction(client, rtx)
	result, rethR := getResult(client, rHash)
	if !result {
		fmt.Println("sendRouterContract getResult", err)
		return nil, false
	}

	input := packInput(mapAbi, "approve", "approve", rethR, new(big.Int).SetUint64(1000000000000000000))
	aHash := sendRouterTransaction(client, addr, basecontract.mapTR, transactOpts.Value, key, input)
	result, _ = getResult(client, aHash)
	if !result {
		fmt.Println("sendRouterContract getResult", err)
		return nil, false
	}
	return &RouterContract{routerAddr: routerAddr, RTran: RTran, rethR: rethR}, result
}

type RouterContract struct {
	routerAddr common.Address
	RTran      *Token
	rethR      common.Address
}

func sendBaseContract(transactOpts *bind.TransactOpts, contractBackend *backends.SimulatedBackend, client *etrueclient.Client) (*BaseContract, bool) {
	weth, wtx, _, err := TokenE.DeployTokene(transactOpts, contractBackend)
	fac, ftx, _, err := tokenf.DeployTokenf(transactOpts, contractBackend, addr)
	mapT, mtx, _, err := tokenm.DeployTokenm(transactOpts, contractBackend)
	mapTran, err := tokenm.NewTokenm(mapT, contractBackend)
	contractBackend.Commit()
	if err != nil {
		fmt.Println("sendBaseContract", err)
	}
	wHash := sendContractTransaction(client, wtx)
	fHash := sendContractTransaction(client, ftx)
	mHash := sendContractTransaction(client, mtx)

	result, wethR := getResult(client, wHash)
	result1, facR := getResult(client, fHash)
	result2, mapTR := getResult(client, mHash)

	if !result || !result1 || !result2 {
		return nil, false
	}
	return &BaseContract{weth: weth, fac: fac, mapT: mapT, mapTran: mapTran,
		wethR: wethR, facR: facR, mapTR: mapTR}, true
}

type BaseContract struct {
	weth, fac, mapT    common.Address
	mapTran            *tokenm.Tokenm
	wethR, facR, mapTR common.Address
}

func sendContractTransaction(client *etrueclient.Client, txTran *types.Transaction) common.Hash {
	// Ensure a valid value field and resolve the account nonce
	from := addr
	if nonce == 0 {
		var err error
		nonce, err = client.PendingNonceAt(context.Background(), from)
		if err != nil {
			fmt.Println(err)
		}
		if nonce == 0 {
			nonce = 1
		}
	} else {
		nonce = nonce + 1
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		fmt.Println(err)
	}

	gasLimit := txTran.Gas()
	value := txTran.Value()

	if gasLimit < 21000 {
		// If the contract surely has code (or code is not needed), estimate the transaction
		msg := truechain.CallMsg{From: from, To: txTran.To(), GasPrice: gasPrice, Value: value, Data: txTran.Data()}
		gasLimit, err = client.EstimateGas(context.Background(), msg)
		if err != nil {
			fmt.Println("Contract exec failed", err)
		}
	}

	// Create the transaction, sign it and schedule it for execution
	if txTran.To() == nil {

	}
	var tx *types.Transaction
	if txTran.To() == nil {
		tx = types.NewContractCreation(nonce, value, gasLimit, gasPrice, txTran.Data())
	} else {
		tx = types.NewTransaction(nonce, *txTran.To(), value, gasLimit, gasPrice, txTran.Data())
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("TX data nonce ", nonce, " transfer value ", value, " gasLimit ", gasLimit, " gasPrice ", gasPrice, " chainID ", chainID)

	signedTx, err := types.SignTx(tx, types.NewTIP1Signer(chainID), key)
	if err != nil {
		fmt.Println(err)
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		fmt.Println("SendTransaction", err)
	}

	return signedTx.Hash()
}

func sendRouterTransaction(client *etrueclient.Client, from, toAddress common.Address, value *big.Int, privateKey *ecdsa.PrivateKey, input []byte) common.Hash {
	// Ensure a valid value field and resolve the account nonce
	nonce = nonce + 1

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		fmt.Println(err)
	}

	gasLimit := uint64(2100000) // in units
	// If the contract surely has code (or code is not needed), estimate the transaction
	msg := truechain.CallMsg{From: from, To: &toAddress, GasPrice: gasPrice, Value: value, Data: input}
	gasLimit, err = client.EstimateGas(context.Background(), msg)
	if err != nil {
		fmt.Println("Contract exec failed", err)
	}
	if gasLimit < 1 {
		gasLimit = 866328
	}

	// Create the transaction, sign it and schedule it for execution
	tx := types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, input)

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("TX data nonce ", nonce, " transfer value ", value, " gasLimit ", gasLimit, " gasPrice ", gasPrice, " chainID ", chainID)

	signedTx, err := types.SignTx(tx, types.NewTIP1Signer(chainID), privateKey)
	if err != nil {
		fmt.Println(err)
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		fmt.Println(err)
	}

	return signedTx.Hash()
}

func getResult(conn *etrueclient.Client, txHash common.Hash) (bool, common.Address) {
	fmt.Println("Please waiting ", " txHash ", txHash.String())

	count := 0
	for {
		time.Sleep(time.Millisecond * 200)
		_, isPending, err := conn.TransactionByHash(context.Background(), txHash)
		if err != nil {
			fmt.Println(err)
			return false, common.Address{}
		}
		count++
		if !isPending {
			break
		}
		if count >= 40 {
			fmt.Println("Please use querytx sub command query later.")
			os.Exit(0)
		}
	}

	return queryTx(conn, txHash, false)
}

func queryTx(conn *etrueclient.Client, txHash common.Hash, pending bool) (bool, common.Address) {
	if pending {
		_, isPending, err := conn.TransactionByHash(context.Background(), txHash)
		if err != nil {
			fmt.Println(err)
		}
		if isPending {
			println("In tx_pool no validator  process this, please query later")
			os.Exit(0)
		}
	}

	receipt, err := conn.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		fmt.Println(err)
	}

	if receipt.Status == types.ReceiptStatusSuccessful {
		block, err := conn.BlockByHash(context.Background(), receipt.BlockHash)
		if err != nil {
			fmt.Println(err)
		}

		fmt.Println("Transaction Success", " block Number", receipt.BlockNumber.Uint64(), " block txs", len(block.Transactions()), "blockhash", block.Hash().Hex())
		return true, receipt.ContractAddress
	} else if receipt.Status == types.ReceiptStatusFailed {
		fmt.Println("Transaction Failed ", " Block Number", receipt.BlockNumber.Uint64())
	}
	return false, common.Address{}
}

func dialConn() (*etrueclient.Client, string) {
	ip := "47.92.246.187"
	//ip := "127.0.0.1"

	port := 8545

	url := fmt.Sprintf("http://%s", fmt.Sprintf("%s:%d", ip, port))
	// Create an IPC based RPC connection to a remote node
	// "http://39.100.97.129:8545"
	//url = "https://rpc.truescan.network/testnet"
	conn, err := etrueclient.Dial(url)
	if err != nil {
		log.Error("dialConn", "Failed to connect to the Truechain client: %v", err)
	}
	return conn, url
}

func printBaseInfo(conn *etrueclient.Client, url string) *types.Header {
	header, err := conn.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Error("printBaseInfo", "err", err)
	}

	if common.IsHexAddress(addr.Hex()) {
		fmt.Println("Connect url ", url, " current number ", header.Number.String(), " address ", addr.Hex())
	} else {
		fmt.Println("Connect url ", url, " current number ", header.Number.String())
	}

	return header
}

func PrintBalance(conn *etrueclient.Client, from common.Address) {
	balance, err := conn.BalanceAt(context.Background(), from, nil)
	if err != nil {
		log.Error("PrintBalance", "err", err)
	}
	fbalance := new(big.Float)
	fbalance.SetString(balance.String())
	trueValue := new(big.Float).Quo(fbalance, big.NewFloat(math.Pow10(18)))

	fmt.Println("Your wallet valid balance is ", trueValue, "'true ", " balance ", balance, "'true ")
}
