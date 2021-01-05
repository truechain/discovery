// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"math"
	"time"
	"truechain/discovery/common"
	"truechain/discovery/crypto"
	"truechain/discovery/log"
	"truechain/discovery/metrics"

	"truechain/discovery/consensus"
	"truechain/discovery/core/state"
	"truechain/discovery/core/types"
	"truechain/discovery/core/vm"
	"truechain/discovery/params"

	"math/big"
)

var (
	blockExecutionTxTimer = metrics.NewRegisteredTimer("chain/state/executiontx", nil)
	blockFinalizeTimer    = metrics.NewRegisteredTimer("chain/state/finalize", nil)
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config *params.ChainConfig // Chain configuration options
	bc     *BlockChain         // Canonical block chain
	engine consensus.Engine    // Consensus engine used for block rewards
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(config *params.ChainConfig, bc *BlockChain, engine consensus.Engine) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		engine: engine,
	}
}

// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (fp *StateProcessor) Process(block *types.Block, statedb *state.StateDB,
	cfg vm.Config) (types.Receipts, []*types.Log, uint64, *types.ChainReward, error) {
	t0 := time.Now()

	if block.Transactions().Len() != 0 {
		log.Info("Process:", "block ", block.Number(), "txs count", block.Transactions().Len())
	}

	if true {
		var (
			feeAmount = big.NewInt(0)
			header    = block.Header()
		)

		parallelBlock := NewParallelBlock(block, statedb, fp.config, fp.bc, cfg, feeAmount)
		receipts, allLogs, usedGas, err := parallelBlock.Process()
		if err != nil {
			return nil, nil, 0, nil, err
		}

		d0 := time.Since(t0)
		t0 = time.Now()
		// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
		_, infos, err := fp.engine.Finalize(fp.bc, header, statedb, block.Transactions(), receipts, feeAmount, false)
		if err != nil {
			return nil, nil, 0, nil, err
		}

		if block.Transactions().Len() != 0 {
			log.Info("Process:", "block ", block.Number(), "txs", block.Transactions().Len(),
				"groups", len(parallelBlock.executionGroups), "execute", common.PrettyDuration(d0),
				"finalize", common.PrettyDuration(time.Since(t0)))
		}

		return receipts, allLogs, usedGas, infos, nil
	} else {

		var (
			receipts  types.Receipts
			usedGas   = new(uint64)
			feeAmount = big.NewInt(0)
			header    = block.Header()
			allLogs   []*types.Log
			gp        = new(GasPool).AddGas(block.GasLimit())
		)
		// Iterate over and process the individual transactions
		for i, tx := range block.Transactions() {
			statedb.Prepare(tx.Hash(), block.Hash(), i)
			receipt, err := ApplyTransaction(fp.config, fp.bc, gp, statedb, header, tx, usedGas, feeAmount, cfg)
			if err != nil {
				return nil, nil, 0, nil, err
			}
			receipts = append(receipts, receipt)
			allLogs = append(allLogs, receipt.Logs...)
		}

		//if block.Number().Cmp(number) == 0  {
		//	fmt.Println("merkle root ( local: %x )", statedb.IntermediateRoot(true))
		//}
		d0 := time.Since(t0)
		t0 = time.Now()
		// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
		_, infos, err := fp.engine.Finalize(fp.bc, header, statedb, block.Transactions(), receipts, feeAmount, false)
		if err != nil {
			return nil, nil, 0, nil, err
		}

		//if block.Number().Cmp(number) == 0  {
		//	fmt.Println("merkle root (remote: %x local: %x local header: %x)", block.Header().Root, statedb.IntermediateRoot(true), header.Root)
		//}

		if block.Transactions().Len() != 0 {
			log.Info("Process:", "block ", block.Number(), "txs count", block.Transactions().Len(),
				"execute", common.PrettyDuration(d0), "finalize", common.PrettyDuration(time.Since(t0)))
		}

		return receipts, allLogs, *usedGas, infos, nil
	}
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(config *params.ChainConfig, bc ChainContext, gp *GasPool,
	statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, feeAmount *big.Int, cfg vm.Config) (*types.Receipt, error) {
	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))
	if err != nil {
		return nil, err
	}
	if err := types.ForbidAddress(msg.From()); err != nil {
		return nil, err
	}
	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc, nil, nil)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)
	// Apply the transaction to the current state (included in the env)
	result, err := ApplyMessage(vmenv, msg, gp)

	if err != nil {
		return nil, err
	}
	// Update the state with pending changes
	var root []byte

	statedb.Finalise(true)

	*usedGas += result.UsedGas
	gasFee := new(big.Int).Mul(new(big.Int).SetUint64(result.UsedGas), msg.GasPrice())
	feeAmount.Add(gasFee, feeAmount)
	if msg.Fee() != nil {
		feeAmount.Add(msg.Fee(), feeAmount) //add fee
	}

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing wether the root touch-delete accounts.
	receipt := types.NewReceipt(root, result.Failed(), *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = result.UsedGas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	receipt.BlockHash = statedb.BlockHash()
	receipt.BlockNumber = header.Number
	receipt.TransactionIndex = uint(statedb.TxIndex())

	return receipt, err
}

// ReadTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the result
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ReadTransaction(config *params.ChainConfig, bc ChainContext,
	statedb *state.StateDB, header *types.Header, tx *types.Transaction, cfg vm.Config) ([]byte, uint64, error) {

	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))

	msgCopy := types.NewMessage(msg.From(), msg.To(), msg.Payment(), 0, msg.Value(), msg.Fee(), msg.Gas(), msg.GasPrice(), msg.Data(), false)

	if err != nil {
		return nil, 0, err
	}
	if err := types.ForbidAddress(msgCopy.From()); err != nil {
		return nil, 0, err
	}
	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msgCopy, header, bc, nil, nil)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)
	// Apply the transaction to the current state (included in the env)
	gp := new(GasPool).AddGas(math.MaxUint64)
	result, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, 0, err
	}

	return result.ReturnData, result.UsedGas, err
}

// ApplyTransactionMsg attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransactionMsg(config *params.ChainConfig, bc ChainContext, gp *GasPool,
	statedb *state.StateDB, header *types.Header, msg *types.Message, tx *types.Transaction, usedGas *uint64, feeAmount *big.Int, cfg vm.Config) (*types.Receipt, error) {
	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc, nil, nil)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)
	// Apply the transaction to the current state (included in the env)
	result, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, err
	}
	// Update the state with pending changes
	var root []byte

	//statedb.Finalise(true)
	statedb.FinaliseEmptyObjects()

	*usedGas += result.UsedGas
	gasFee := new(big.Int).Mul(new(big.Int).SetUint64(result.UsedGas), msg.GasPrice())
	feeAmount.Add(gasFee, feeAmount)
	if msg.Fee() != nil {
		feeAmount.Add(msg.Fee(), feeAmount) //add fee
	}

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing wether the root touch-delete accounts.
	receipt := types.NewReceipt(root, result.Failed(), *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = result.UsedGas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	return receipt, err
}
