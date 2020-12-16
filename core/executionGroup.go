package core

import (
	"truechain/discovery/common"
	"truechain/discovery/core/state"
	"truechain/discovery/core/types"
)

type ExecutionGroup struct {
	id            int
	header        *types.Header
	transactions  types.Transactions
	startTrxIndex int
	statedb       *state.StateDB

	// transaction execution result
	trxHashToResultMap map[common.Hash]*TrxResult
	err                error
	errTxIndex         int
	usedGas            uint64
}

type TrxResult struct {
	receipt          *types.Receipt
	logs             []*types.Log
	touchedAddresses *state.TouchedAddressObject
	usedGas          uint64
}

func NewTrxResult(receipt *types.Receipt, logs []*types.Log, touchedAddresses *state.TouchedAddressObject, usedGas uint64) *TrxResult {
	return &TrxResult{receipt: receipt, logs: logs, touchedAddresses: touchedAddresses, usedGas: usedGas}
}

func NewExecutionGroup() *ExecutionGroup {
	return &ExecutionGroup{}
}

func (e *ExecutionGroup) Transactions() types.Transactions {
	return e.transactions
}

func (e *ExecutionGroup) SetTransactions(transactions types.Transactions) {
	e.transactions = transactions
}

func (e *ExecutionGroup) Header() *types.Header {
	return e.header
}

func (e *ExecutionGroup) SetHeader(header *types.Header) {
	e.header = header
}

func (e *ExecutionGroup) AddTransaction(trx *types.Transaction) {
	e.transactions = append(e.transactions, trx)
}

func (e *ExecutionGroup) AddTransactions(transactions types.Transactions) {
	e.transactions = append(e.transactions, transactions...)
}

func (e *ExecutionGroup) SetId(groupId int) {
	e.id = groupId
}

func (e *ExecutionGroup) SetStartTrxPos(index int) {
	e.startTrxIndex = index
}

func (e *ExecutionGroup) SetStatedb(statedb *state.StateDB) {
	e.statedb = statedb
}

func (e *ExecutionGroup) reuseTxResults(txsToReuse []TxWithOldGroup, conflictGroups map[int]*ExecutionGroup) {
	stateObjsFromOtherGroup := make(map[int]map[common.Address]*state.StateObjectToReuse)

	for gId, _ := range conflictGroups {
		stateObjsFromOtherGroup[gId] = make(map[common.Address]*state.StateObjectToReuse)
	}

	for i := len(txsToReuse) - 1; i >= 0; i-- {
		txHash := txsToReuse[i].txHash
		oldGroupId := txsToReuse[i].oldGroupId

		if result, ok := conflictGroups[oldGroupId].trxHashToResultMap[txHash]; ok {
			appendStateObjToReuse(stateObjsFromOtherGroup[oldGroupId], result.touchedAddresses)

			e.statedb.CopyTxJournalFromOtherDB(conflictGroups[oldGroupId].statedb, txHash)

			e.trxHashToResultMap[txHash] = result
		}
	}

	for gId, stateObjsMap := range stateObjsFromOtherGroup {
		e.statedb.CopyStateObjFromOtherDB(conflictGroups[gId].statedb, stateObjsMap)
	}

	e.statedb.Finalise(true)
}

func appendStateObjToReuse(stateObjsToReuse map[common.Address]*state.StateObjectToReuse, touchedAddr *state.TouchedAddressObject) {
	for addr, op := range touchedAddr.AccountOp() {
		if op {
			if stateObj, ok := stateObjsToReuse[addr]; !ok {
				stateObj = state.NewStateObjectToReuse(addr, nil, true)
				stateObjsToReuse[addr] = stateObj
			} else {
				stateObj.ReuseData = true
			}
		}
	}
	for storage, op := range touchedAddr.StorageOp() {
		if op {
			addr := storage.AccountAddress
			if stateObj, ok := stateObjsToReuse[addr]; !ok {
				stateObj = state.NewStateObjectToReuse(addr, nil, false)
				stateObjsToReuse[addr] = stateObj
			} else {
				stateObj.Keys = append(stateObj.Keys, storage.Key)
			}
		}
	}
}