package token

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"truechain/discovery"
	"truechain/discovery/accounts/abi"
	"truechain/discovery/common"
	"truechain/discovery/common/hexutil"
	"truechain/discovery/contracts/uniswap/TokenE"
	tokenf "truechain/discovery/contracts/uniswap/TokenF"
	"truechain/discovery/contracts/uniswap/tokenm"
	"truechain/discovery/core/types"

	"truechain/discovery/accounts/abi/bind"
	"truechain/discovery/accounts/abi/bind/backends"
	"truechain/discovery/crypto"
	"truechain/discovery/log"
)

func init() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(false))))
}

func TestUniswap(t *testing.T) {
	contractBackend := backends.NewSimulatedBackend(types.GenesisAlloc{
		addr:     {Balance: new(big.Int).SetUint64(10000000000000000000)},
		testAddr: {Balance: big.NewInt(100000000000000)}},
		100000000)
	transactOpts := bind.NewKeyedTransactor(key)

	weth, _, _, err := TokenE.DeployTokene(transactOpts, contractBackend)
	if err != nil {
		t.Fatalf("can't DeployContract: %v", err)
	}
	fac, _, _, err := tokenf.DeployTokenf(transactOpts, contractBackend, addr)
	if err != nil {
		t.Fatalf("can't DeployContract Factory: %v", err)
	}
	mapT, _, _, err := tokenm.DeployTokenm(transactOpts, contractBackend)
	if err != nil {
		t.Fatalf("can't DeployContract: %v", err)
	}
	mapTran, err := tokenm.NewTokenm(mapT, contractBackend)
	if err != nil {
		t.Fatalf("can't NewContract: %v", err)
	}

	contractBackend.Commit()

	balance, err := mapTran.BalanceOf(nil, addr)
	if err != nil {
		log.Error("Failed to retrieve token ", "name: %v", err)
	}
	fmt.Println("addr balance BalanceOf", balance)

	// Deploy the ENS registry
	unisWapAddr, _, _, err := DeployToken(transactOpts, contractBackend, fac, weth)
	if err != nil {
		t.Fatalf("can't DeployContract: %v", err)
	}
	_, err = mapTran.Approve(transactOpts, unisWapAddr, new(big.Int).SetUint64(10000000000000))
	if err != nil {
		t.Fatalf("can't NewContract: %v", err)
	}
	contractBackend.Commit()

	ens, err := NewToken(unisWapAddr, contractBackend)
	if err != nil {
		t.Fatalf("can't NewContract: %v", err)
	}
	tik := new(big.Int).SetUint64(10000000000000000)
	tik1 := new(big.Int).SetUint64(1000000000000)

	balance, err = mapTran.Allowance(nil, addr, unisWapAddr)
	name, err := mapTran.Name(nil)
	fmt.Println("balance ", balance, "name", name, " unisWapAddr ", unisWapAddr.String())
	transactOpts.Value = new(big.Int).SetUint64(1000000000000000000)
	fmt.Println(mapT.String(), " ", addr.String(), " fac ", fac.String(), " eth ", weth.String())
	fmt.Println(hexutil.Encode(tik.Bytes()), " ", hexutil.Encode(tik1.Bytes()))
	backends.SimulateDebug = false
	_, err = ens.AddLiquidityETH(transactOpts, mapT, tik, tik, tik1, addr, new(big.Int).SetUint64(1699658290))
	if err != nil {
		t.Fatalf("can't NewContract AddLiquidityETH : %v", err)
	}
	contractBackend.Commit()
}

func TestTx(t *testing.T) {
	client, url := dialConn()
	printBaseInfo(client, url)
	addr := common.HexToAddress("0x854dc6f847472964fc7f2b3f7380d60f7284cae5")
	PrintBalance(client, addr)

	fmt.Println("addr ", addr.String())
	var err error
	nonce, err = client.PendingNonceAt(context.Background(), addr)
	if err != nil {
		fmt.Println("PendingNonceAt", err)
	}
	fmt.Println("PendingNonceAt", nonce)

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("PendingNonceAt", nonce, " chainID ", chainID)

	dataS := "2e1a7d4d0000000000000000000000000000000000000000000000000de0b6b3a7640000"
	data, err := hex.DecodeString(dataS)
	if err != nil {
		fmt.Println(err)
	}
	transaction := types.NewTransaction(nonce, common.HexToAddress("0x9a11fe0a8dc1d8fd7067eed03ff9415d6a3b75c2"), nil, 60000, new(big.Int).SetUint64(1000000000), data)

	msg := truechain.CallMsg{From: addr, To: transaction.To(), GasPrice: transaction.GasPrice(), Value: transaction.Value(), Data: transaction.Data()}
	gasLimit, err := client.EstimateGas(context.Background(), msg)
	if err != nil {
		fmt.Println("Contract exec failed", err)
	}
	fmt.Println("gasLimit ", gasLimit)
}

func TestNode(t *testing.T) {
	client, url := dialConn()
	printBaseInfo(client, url)
	pay, _ := crypto.HexToECDSA("142778b7a14777c8d3d0f5d7ed913ab87dd0ca8fd008c2e76567c5761424c013")
	key, _ := crypto.HexToECDSA("1ab7e07b0da4498d222318f02d7e28a9c36934a5039e39f88d39a9a5f0522859")
	addr := crypto.PubkeyToAddress(pay.PublicKey)
	keyAddr := crypto.PubkeyToAddress(key.PublicKey)
	PrintBalance(client, addr)
	PrintBalance(client, keyAddr)

	fmt.Println("addr ", addr.String(), " keyAddr ", keyAddr.String())
	var err error
	nonce, err = client.PendingNonceAt(context.Background(), keyAddr)
	if err != nil {
		fmt.Println("PendingNonceAt", err)
	}
	fmt.Println("PendingNonceAt", nonce)

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("PendingNonceAt", nonce, " chainID ", chainID)

	//sender := types.NewTIP1Signer(chainID)
	dataS := "a9059cbb000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002dc6c0"
	data, err := hex.DecodeString(dataS)
	if err != nil {
		fmt.Println(err)
	}
	transaction := types.NewTransaction_Payment(nonce, common.HexToAddress("0x35a9330045d7ee57af5a977f0ac2d0632437b418"), nil, new(big.Int).SetUint64(0), 60000, new(big.Int).SetUint64(1000000000), data, addr)

	msg := truechain.CallMsg{From: keyAddr, To: transaction.To(), GasPrice: transaction.GasPrice(), Value: transaction.Value(), Data: transaction.Data()}
	gasLimit, err := client.EstimateGas(context.Background(), msg)
	if err != nil {
		fmt.Println("Contract exec failed", err)
	}
	fmt.Println("gasLimit ", gasLimit)
	//transaction, err = types.SignTx(transaction, sender, key)
	//if err != nil {
	//	fmt.Println(err)
	//}
	//transaction, err = types.SignTx_Payment(transaction, sender, pay)
	//if err != nil {
	//	fmt.Println("SignTx_Payment ",err)
	//}
	//err = client.SendTransaction(context.Background(), transaction)
	//if err != nil {
	//	fmt.Println(err)
	//}
	//getResult(client,transaction.Hash())
	fmt.Println(nonce)
}

func TestDialNode(t *testing.T) {
	client, url := dialConn()
	printBaseInfo(client, url)
	PrintBalance(client, addr)

	contractBackend := backends.NewSimulatedBackend(types.GenesisAlloc{
		addr:     {Balance: new(big.Int).SetUint64(10000000000000000000)},
		testAddr: {Balance: big.NewInt(100000000000000)}},
		100000000)
	transactOpts := bind.NewKeyedTransactor(key)

	basecontract, result := sendBaseContract(transactOpts, contractBackend, client)
	if !result {
		fmt.Println("sendBaseContract failed")
		return
	}
	routercontract, result := sendRouterContract(transactOpts, contractBackend, client, basecontract)
	if !result {
		fmt.Println("sendRouterContract failed")
		return
	}
	simulateRouter(transactOpts, contractBackend, basecontract, routercontract)

	tik := new(big.Int).SetUint64(10000000000000000)
	tik1 := new(big.Int).SetUint64(1000000000000)

	input := packInput(routerAbi, "addLiquidityETH", "addLiquidityETH", basecontract.mapTR, tik, tik, tik1, addr, new(big.Int).SetUint64(1699658290))
	aHash := sendRouterTransaction(client, addr, routercontract.rethR, transactOpts.Value, key, input)
	result, _ = getResult(client, aHash)
	fmt.Println("over result", result)
}

func TestPriv(t *testing.T) {
	test, _ := crypto.HexToECDSA("647eeeb80193a47a02d31939af29efa006dbe6db45c8806af764c18b262bb90b")
	mkey, _ := crypto.HexToECDSA("5e6ea3e3ba8a3d8940088247eda01a0909320f729ae3afcdc5747b2ced1ac460")
	bftkey, _ := crypto.HexToECDSA("c1581e25937d9ab91421a3e1a2667c85b0397c75a195e643109938e987acecfc")

	testAddrAddr := crypto.PubkeyToAddress(test.PublicKey)
	add3Addr := crypto.PubkeyToAddress(mkey.PublicKey)
	bftkeyAddr := crypto.PubkeyToAddress(bftkey.PublicKey)
	fmt.Println(testAddrAddr.String(), " ", add3Addr.String())
	fmt.Println(bftkeyAddr.String(), " ", hex.EncodeToString(crypto.FromECDSAPub(&bftkey.PublicKey)))
}

func TestNewToken(t *testing.T) {
	sig := "_addLiquidity(address,address,uint,uint,uint,uint)"
	id := crypto.Keccak256([]byte(sig))[:4]
	fmt.Println(hexutil.Encode(id))

	sig = "addLiquidityETH(address,uint256,uint256,uint256,address,uint256)"
	id = crypto.Keccak256([]byte(sig))[:4]
	fmt.Println(hexutil.Encode(id))
	sig = "pairFor(address,address,address)"
	id = crypto.Keccak256([]byte(sig))[:4]
	fmt.Println(hexutil.Encode(id))
}

func TestParseDepositInput(t *testing.T) {
	input := "fb3bdb41000000000000000000000000000000000000000000108b2a2c280290940000000000000000000000000000000000000000000000000000000000000000000080000000000000000000000000b634058bd3ac146b128824e4b9ab59561b0568b9000000000000000000000000000000000000000000000000000000005f3f6a000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000c02aaa39b223fe8d0a0e5c4f27ead9083c756cc2000000000000000000000000ed1199093b1abd07a368dd1c0cdc77d8517ba2a0"
	inputData, _ := hex.DecodeString(input)
	abiStaking, _ := abi.JSON(strings.NewReader(Json))
	methodName, err := abiStaking.MethodById(inputData)
	data := inputData[4:]
	args := struct {
		AmountOut *big.Int
		Path      []common.Address
		To        common.Address
		Deadline  *big.Int
	}{}
	method, _ := abiStaking.Methods[methodName.Name]

	err = method.Inputs.Unpack(&args, data)
	if err != nil {
		log.Error("Unpack deposit pubkey error", "err", err)
	}
	fmt.Println("AmountOut ", hex.EncodeToString(args.AmountOut.Bytes()), " Deadline ", hex.EncodeToString(args.Deadline.Bytes()), " ", len(args.Path), " ", args.To.String())
}

var abiStaking, _ = abi.JSON(strings.NewReader(Json))

func TestGatAddress(t *testing.T) {
	b := common.HexToAddress("0xa9A2CbA5d5d16DE370375B42662F3272279B2b89")

	cb := crypto.CreateAddress(b, uint64(8))

	fmt.Println(cb.String())
}

func TestUnpack(t *testing.T) {
	args := struct {
		Pubkey []byte
		Fee    *big.Int
		Value  *big.Int
	}{}
	// 5d322ae8
	inputstr := "5d322ae8000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000003e8000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000004050863ad64a87ae8a2fe83c1af1a8403cb53f53e486d8511dad8a04887e5b23522cd470243453a299fa9e77237716103abc11a1df38855ed6f2ee187e9c582ba6"
	input, e := hex.DecodeString(inputstr)
	if e != nil {
		fmt.Println(e)
	}
	method, ok := abiStaking.Methods["deposit"]
	if !ok {
		fmt.Println("cann't find")
	}
	input = input[4:]
	err := method.Inputs.Unpack(&args, input)
	if err != nil {
		fmt.Println("Unpack deposit pubkey error", err)
	}
	// vpk,e2 := hex.DecodeString("0450863ad64a87ae8a2fe83c1af1a8403cb53f53e486d8511dad8a04887e5b23522cd470243453a299fa9e77237716103abc11a1df38855ed6f2ee187e9c582ba6")
	// if e2 != nil {
	// 	fmt.Println("e2:",e2)
	// }
	if _, err := crypto.UnmarshalPubkey(args.Pubkey); err != nil {
		fmt.Println("invalid pk,err:", err)
	}
	fmt.Println("pk:", hex.EncodeToString(args.Pubkey))
	fmt.Println("fee", args.Fee.String())
	fmt.Println("Value", args.Value.String())
	fmt.Println("finish")
}

const Json = `
[
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "_factory",
				"type": "address"
			},
			{
				"internalType": "address",
				"name": "_WETH",
				"type": "address"
			}
		],
		"stateMutability": "nonpayable",
		"type": "constructor"
	},
	{
		"inputs": [],
		"name": "WETH",
		"outputs": [
			{
				"internalType": "address",
				"name": "",
				"type": "address"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "tokenA",
				"type": "address"
			},
			{
				"internalType": "address",
				"name": "tokenB",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "amountADesired",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountBDesired",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountAMin",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountBMin",
				"type": "uint256"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			}
		],
		"name": "addLiquidity",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "amountA",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountB",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "liquidity",
				"type": "uint256"
			}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "token",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "amountTokenDesired",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountTokenMin",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountETHMin",
				"type": "uint256"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			}
		],
		"name": "addLiquidityETH",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "amountToken",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountETH",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "liquidity",
				"type": "uint256"
			}
		],
		"stateMutability": "payable",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "factory",
		"outputs": [
			{
				"internalType": "address",
				"name": "",
				"type": "address"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amountOut",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "reserveIn",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "reserveOut",
				"type": "uint256"
			}
		],
		"name": "getAmountIn",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "amountIn",
				"type": "uint256"
			}
		],
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amountIn",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "reserveIn",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "reserveOut",
				"type": "uint256"
			}
		],
		"name": "getAmountOut",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "amountOut",
				"type": "uint256"
			}
		],
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amountOut",
				"type": "uint256"
			},
			{
				"internalType": "address[]",
				"name": "path",
				"type": "address[]"
			}
		],
		"name": "getAmountsIn",
		"outputs": [
			{
				"internalType": "uint256[]",
				"name": "amounts",
				"type": "uint256[]"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amountIn",
				"type": "uint256"
			},
			{
				"internalType": "address[]",
				"name": "path",
				"type": "address[]"
			}
		],
		"name": "getAmountsOut",
		"outputs": [
			{
				"internalType": "uint256[]",
				"name": "amounts",
				"type": "uint256[]"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amountA",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "reserveA",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "reserveB",
				"type": "uint256"
			}
		],
		"name": "quote",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "amountB",
				"type": "uint256"
			}
		],
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "tokenA",
				"type": "address"
			},
			{
				"internalType": "address",
				"name": "tokenB",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "liquidity",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountAMin",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountBMin",
				"type": "uint256"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			}
		],
		"name": "removeLiquidity",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "amountA",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountB",
				"type": "uint256"
			}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "token",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "liquidity",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountTokenMin",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountETHMin",
				"type": "uint256"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			}
		],
		"name": "removeLiquidityETH",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "amountToken",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountETH",
				"type": "uint256"
			}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "token",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "liquidity",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountTokenMin",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountETHMin",
				"type": "uint256"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			}
		],
		"name": "removeLiquidityETHSupportingFeeOnTransferTokens",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "amountETH",
				"type": "uint256"
			}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "token",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "liquidity",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountTokenMin",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountETHMin",
				"type": "uint256"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			},
			{
				"internalType": "bool",
				"name": "approveMax",
				"type": "bool"
			},
			{
				"internalType": "uint8",
				"name": "v",
				"type": "uint8"
			},
			{
				"internalType": "bytes32",
				"name": "r",
				"type": "bytes32"
			},
			{
				"internalType": "bytes32",
				"name": "s",
				"type": "bytes32"
			}
		],
		"name": "removeLiquidityETHWithPermit",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "amountToken",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountETH",
				"type": "uint256"
			}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "token",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "liquidity",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountTokenMin",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountETHMin",
				"type": "uint256"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			},
			{
				"internalType": "bool",
				"name": "approveMax",
				"type": "bool"
			},
			{
				"internalType": "uint8",
				"name": "v",
				"type": "uint8"
			},
			{
				"internalType": "bytes32",
				"name": "r",
				"type": "bytes32"
			},
			{
				"internalType": "bytes32",
				"name": "s",
				"type": "bytes32"
			}
		],
		"name": "removeLiquidityETHWithPermitSupportingFeeOnTransferTokens",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "amountETH",
				"type": "uint256"
			}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "tokenA",
				"type": "address"
			},
			{
				"internalType": "address",
				"name": "tokenB",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "liquidity",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountAMin",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountBMin",
				"type": "uint256"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			},
			{
				"internalType": "bool",
				"name": "approveMax",
				"type": "bool"
			},
			{
				"internalType": "uint8",
				"name": "v",
				"type": "uint8"
			},
			{
				"internalType": "bytes32",
				"name": "r",
				"type": "bytes32"
			},
			{
				"internalType": "bytes32",
				"name": "s",
				"type": "bytes32"
			}
		],
		"name": "removeLiquidityWithPermit",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "amountA",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountB",
				"type": "uint256"
			}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amountOut",
				"type": "uint256"
			},
			{
				"internalType": "address[]",
				"name": "path",
				"type": "address[]"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			}
		],
		"name": "swapETHForExactTokens",
		"outputs": [
			{
				"internalType": "uint256[]",
				"name": "amounts",
				"type": "uint256[]"
			}
		],
		"stateMutability": "payable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amountOutMin",
				"type": "uint256"
			},
			{
				"internalType": "address[]",
				"name": "path",
				"type": "address[]"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			}
		],
		"name": "swapExactETHForTokens",
		"outputs": [
			{
				"internalType": "uint256[]",
				"name": "amounts",
				"type": "uint256[]"
			}
		],
		"stateMutability": "payable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amountOutMin",
				"type": "uint256"
			},
			{
				"internalType": "address[]",
				"name": "path",
				"type": "address[]"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			}
		],
		"name": "swapExactETHForTokensSupportingFeeOnTransferTokens",
		"outputs": [],
		"stateMutability": "payable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amountIn",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountOutMin",
				"type": "uint256"
			},
			{
				"internalType": "address[]",
				"name": "path",
				"type": "address[]"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			}
		],
		"name": "swapExactTokensForETH",
		"outputs": [
			{
				"internalType": "uint256[]",
				"name": "amounts",
				"type": "uint256[]"
			}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amountIn",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountOutMin",
				"type": "uint256"
			},
			{
				"internalType": "address[]",
				"name": "path",
				"type": "address[]"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			}
		],
		"name": "swapExactTokensForETHSupportingFeeOnTransferTokens",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amountIn",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountOutMin",
				"type": "uint256"
			},
			{
				"internalType": "address[]",
				"name": "path",
				"type": "address[]"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			}
		],
		"name": "swapExactTokensForTokens",
		"outputs": [
			{
				"internalType": "uint256[]",
				"name": "amounts",
				"type": "uint256[]"
			}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amountIn",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountOutMin",
				"type": "uint256"
			},
			{
				"internalType": "address[]",
				"name": "path",
				"type": "address[]"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			}
		],
		"name": "swapExactTokensForTokensSupportingFeeOnTransferTokens",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amountOut",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountInMax",
				"type": "uint256"
			},
			{
				"internalType": "address[]",
				"name": "path",
				"type": "address[]"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			}
		],
		"name": "swapTokensForExactETH",
		"outputs": [
			{
				"internalType": "uint256[]",
				"name": "amounts",
				"type": "uint256[]"
			}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amountOut",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "amountInMax",
				"type": "uint256"
			},
			{
				"internalType": "address[]",
				"name": "path",
				"type": "address[]"
			},
			{
				"internalType": "address",
				"name": "to",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "deadline",
				"type": "uint256"
			}
		],
		"name": "swapTokensForExactTokens",
		"outputs": [
			{
				"internalType": "uint256[]",
				"name": "amounts",
				"type": "uint256[]"
			}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"stateMutability": "payable",
		"type": "receive"
	}
]
`
