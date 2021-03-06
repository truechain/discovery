package core

import (
	"container/list"
	"math/big"
	"runtime"
	"truechain/discovery/log"

	"sort"
	"sync"
	"truechain/discovery/common"
	"truechain/discovery/core/state"
	"truechain/discovery/core/types"
	"truechain/discovery/core/vm"
	"truechain/discovery/params"
)

var (
	associatedAddressMngr = NewAssociatedAddressMngr()
	cpuNum                = runtime.NumCPU()
)

type ParallelBlock struct {
	block                *types.Block
	txInfos              []*txInfo
	executionGroups      map[int]*ExecutionGroup
	associatedAddressMap map[common.Address]*state.TouchedAddressObject
	nextGroupId          int
	statedb              *state.StateDB
	config               *params.ChainConfig
	context              ChainContext
	vmConfig             vm.Config
	feeAmount            *big.Int
}

type grouper struct {
	executionGroupMap      map[int]*ExecutionGroup
	groupWrittenAccountMap map[int]map[common.Address]struct{}
	addrToGroupMap         map[common.Address]int
	groupId                int
	regroup                bool
	maxGroupCount          int
	avgTxCountInGroup      int
	groupTxCountUnderAvg   map[int]struct{}
}

type txInfo struct {
	index   int
	hash    common.Hash
	msg     *types.Message
	tx      *types.Transaction
	groupId int
	result  *TrxResult
}

func newTxInfo(index int, hash common.Hash, msg *types.Message, tx *types.Transaction) *txInfo {
	return &txInfo{index: index, hash: hash, msg: msg, tx: tx}
}

type addressRlpDataPair struct {
	address  common.Address
	stateObj interface{}
	rlpData  []byte
}

func newGrouper(totalTxToGroup int, startGroupId int, regroup bool) *grouper {
	maxGroupCount := cpuNum * 2

	avgTxCountInGroup := (totalTxToGroup + maxGroupCount - 1) / (maxGroupCount)
	return &grouper{
		executionGroupMap:      make(map[int]*ExecutionGroup),
		groupWrittenAccountMap: make(map[int]map[common.Address]struct{}),
		addrToGroupMap:         make(map[common.Address]int),
		groupId:                startGroupId,
		regroup:                regroup,
		maxGroupCount:          maxGroupCount,
		avgTxCountInGroup:      avgTxCountInGroup,
		groupTxCountUnderAvg:   make(map[int]struct{}),
	}
}

func (gp *grouper) groupNewTxInfo(txInfo *txInfo, pb *ParallelBlock) {
	groupsToMerge := make(map[int]struct{})
	groupWrittenAccount := make(map[common.Address]struct{})
	firstGroup := true
	var tmpExecutionGroup *ExecutionGroup
	trxTouchedAddress := pb.getTrxTouchedAddress(txInfo, gp.regroup)

	for addr, op := range trxTouchedAddress.AccountOp() {
		if gId, ok := gp.addrToGroupMap[addr]; ok {
			groupsToMerge[gId] = struct{}{}
		}
		if op {
			groupWrittenAccount[addr] = struct{}{}
			gp.addrToGroupMap[addr] = gp.groupId
		}
	}

	if len(groupsToMerge) == 0 {
		if len(gp.executionGroupMap) < gp.maxGroupCount {
			tmpExecutionGroup = NewExecutionGroup()
			tmpExecutionGroup.addTxInfo(txInfo)
			tmpExecutionGroup.SetHeader(pb.block.Header())
			tmpExecutionGroup.SetId(gp.groupId)
			gp.groupWrittenAccountMap[gp.groupId] = groupWrittenAccount
			gp.executionGroupMap[gp.groupId] = tmpExecutionGroup
			if 1 < gp.avgTxCountInGroup {
				gp.groupTxCountUnderAvg[gp.groupId] = struct{}{}
			}
			gp.groupId++
			return
		} else {
			var pickedGroupId int
			for gId := range gp.groupTxCountUnderAvg {
				pickedGroupId = gId
				break
			}
			groupsToMerge[pickedGroupId] = struct{}{}
		}
	}

	var curGroupId int
	for gId := range groupsToMerge {
		if firstGroup {
			curGroupId = gId
			tmpExecutionGroup = gp.executionGroupMap[gId]
			tmpExecutionGroup.addTxInfo(txInfo)

			for k, v := range groupWrittenAccount {
				gp.groupWrittenAccountMap[gId][k] = v
				gp.addrToGroupMap[k] = gId
			}

			firstGroup = false
		} else {
			tmpExecutionGroup.addTxInfos(gp.executionGroupMap[gId].getTxInfos())
			for k, v := range gp.groupWrittenAccountMap[gId] {
				gp.groupWrittenAccountMap[curGroupId][k] = v
				gp.addrToGroupMap[k] = curGroupId
			}
			delete(gp.executionGroupMap, gId)
			delete(gp.groupWrittenAccountMap, gId)
			delete(gp.groupTxCountUnderAvg, gId)
		}
	}

	if len(gp.executionGroupMap[curGroupId].txInfos) >= gp.avgTxCountInGroup {
		delete(gp.groupTxCountUnderAvg, curGroupId)
	}
}

func NewParallelBlock(block *types.Block, statedb *state.StateDB, config *params.ChainConfig, bc ChainContext, cfg vm.Config, feeAmount *big.Int) *ParallelBlock {
	return &ParallelBlock{
		block:           block,
		txInfos:         make([]*txInfo, block.Transactions().Len()),
		executionGroups: make(map[int]*ExecutionGroup),
		statedb:         statedb,
		config:          config,
		context:         bc,
		vmConfig:        cfg,
		feeAmount:       feeAmount,
	}
}

func (pb *ParallelBlock) group(chForTxInfo chan *txInfo, chForGroup chan bool) {
	tmpExecutionGroupMap := pb.groupTransactionsFromChan(chForTxInfo, false)

	for _, execGroup := range tmpExecutionGroupMap {
		if len(tmpExecutionGroupMap) == 1 {
			execGroup.SetStatedb(pb.statedb)
		} else {
			execGroup.SetStatedb(pb.statedb.Copy())
		}
		pb.executionGroups[execGroup.id] = execGroup

		for _, txInfo := range execGroup.txInfos {
			txInfo.groupId = execGroup.id
		}
	}
	chForGroup <- true
}

func (pb *ParallelBlock) reGroupAndRevert(conflictGroupMaps []map[int]struct{}, conflictTxs map[common.Hash]struct{}) {
	for _, conflictGroupIds := range conflictGroupMaps {
		var txInfos []*txInfo
		conflictGroups := make(map[int]*ExecutionGroup)

		for groupId := range conflictGroupIds {
			txInfos = append(txInfos, pb.executionGroups[groupId].getTxInfos()...)
			conflictGroups[groupId] = pb.executionGroups[groupId]
			delete(pb.executionGroups, groupId)
		}

		txInfos = sortTxInfosByIndex(txInfos)
		tmpExecGroupMap := pb.groupTransactions(txInfos, true)

		for _, group := range tmpExecGroupMap {
			var (
				txsToReuse []*txInfo
				revert     = false
			)

			if len(tmpExecGroupMap) == 1 && len(pb.executionGroups) == 0 && len(conflictGroupMaps) == 1 {
				group.SetStatedb(pb.statedb)
			} else {
				group.SetStatedb(pb.statedb.Copy())
			}
			pb.executionGroups[group.id] = group

			for index, txInfo := range group.txInfos {
				txHash := txInfo.hash
				if !revert {
					if _, ok := conflictTxs[txHash]; ok || txInfo.result == nil {
						revert = true
						group.SetStartTrxPos(index)
						break
					} else {
						txsToReuse = append(txsToReuse, txInfo)
					}
				}
			}

			if revert {
				// revert txInfos which will be re-executed in reversed order
				for i := len(group.txInfos) - 1; i >= group.startTrxIndex; i-- {
					txInfo := group.txInfos[i]
					log.Debug("reGroupAndRevert", "revert tx hash", txInfo.hash.String(),
						"index", txInfo.index, "group", txInfo.groupId, "start", group.startTrxIndex)
					if txInfo.result != nil {
						conflictGroups[txInfo.groupId].statedb.RevertTrxResultByHash(txInfo.hash)
						txInfo.result = nil
					}
					txInfo.groupId = group.id
				}
			} else {
				group.SetStartTrxPos(-1)
			}

			// copy transaction results and state changes from old group which can be reused
			group.reuseTxResults(txsToReuse, conflictGroups)

		}
	}
}

func (pb *ParallelBlock) groupTransactions(txInfos []*txInfo, regroup bool) map[int]*ExecutionGroup {
	grouper := newGrouper(len(txInfos), pb.nextGroupId, regroup)

	for _, txInfo := range txInfos {
		grouper.groupNewTxInfo(txInfo, pb)
	}

	for _, group := range grouper.executionGroupMap {
		group.txInfos = sortTxInfosByIndex(group.txInfos)
	}
	pb.nextGroupId = grouper.groupId

	return grouper.executionGroupMap
}

func (pb *ParallelBlock) groupTransactionsFromChan(ch chan *txInfo, regroup bool) map[int]*ExecutionGroup {
	grouper := newGrouper(pb.block.Transactions().Len(), pb.nextGroupId, regroup)

	for tx := range ch {
		grouper.groupNewTxInfo(tx, pb)
	}

	for _, group := range grouper.executionGroupMap {
		group.txInfos = sortTxInfosByIndex(group.txInfos)
	}
	pb.nextGroupId = grouper.groupId

	return grouper.executionGroupMap
}

func (pb *ParallelBlock) getTrxTouchedAddress(txInfo *txInfo, regroup bool) *state.TouchedAddressObject {
	if regroup {
		if result := txInfo.result; result != nil {
			return result.touchedAddresses
		}
	}

	touchedAddressObj := state.NewTouchedAddressObject()
	msg := txInfo.msg

	if msg.Payment() != params.EmptyAddress {
		touchedAddressObj.AddAccountOp(msg.Payment(), true)
	}
	touchedAddressObj.AddAccountOp(msg.From(), true)

	if to := msg.To(); to != nil {
		if associatedAddressObj, ok := pb.associatedAddressMap[*to]; ok {
			touchedAddressObj.Merge(associatedAddressObj)
		} else {
			touchedAddressObj.AddAccountOp(*to, true)
		}
	}

	return touchedAddressObj
}

type conflictInfo struct {
	conflictGroups []map[int]struct{}
	conflictTxs    map[common.Hash]struct{}
}

func (pb *ParallelBlock) checkGroupsConflict(ch0 chan *txInfo, ch1 chan *conflictInfo) {
	var conflictGroups []map[int]struct{}
	conflictTxs := make(map[common.Hash]struct{})
	addrGroupIdsMap := make(map[common.Address]map[int]struct{})
	txInfos := make([]*txInfo, len(pb.txInfos))
	nextIndex := 0

	for tx := range ch0 {
		txInfos[tx.index] = tx
		if tx.index == nextIndex {
			for ; nextIndex < len(txInfos) && txInfos[nextIndex] != nil; nextIndex++ {
				txInfo := txInfos[nextIndex]
				var touchedAddressObj *state.TouchedAddressObject = nil
				trxHash := txInfo.hash
				curTrxGroup := txInfo.groupId
				touchedAddressObj = pb.getTrxTouchedAddress(txInfo, true)

				for addr, op := range touchedAddressObj.AccountOp() {
					if groupIds, ok := addrGroupIdsMap[addr]; ok {
						if _, ok := groupIds[curTrxGroup]; !ok {
							groupIds[curTrxGroup] = struct{}{}
							conflictTxs[trxHash] = struct{}{}
						}
					} else if op {
						groupSet := make(map[int]struct{})
						groupSet[curTrxGroup] = struct{}{}
						addrGroupIdsMap[addr] = groupSet
					}
				}
			}
		}
		if nextIndex == len(txInfos) {
			break
		}
	}

	groupsList := list.New()
	for _, groups := range addrGroupIdsMap {
		groupsList.PushBack(groups)
	}
	for i := groupsList.Front(); i != nil; i = i.Next() {
		groups := i.Value.(map[int]struct{})
		if len(groups) <= 1 {
			continue
		}

		for i := len(conflictGroups) - 1; i >= 0; i-- {
			conflictGroupId := conflictGroups[i]
			if overlapped(conflictGroupId, groups) {
				for k := range conflictGroupId {
					groups[k] = struct{}{}
				}
				conflictGroups = append(conflictGroups[:i], conflictGroups[i+1:]...)
			}
		}

		conflictGroups = append(conflictGroups, groups)
	}

	if len(conflictGroups) != 0 {
		log.Debug("checkConflict", "conflictGroups", conflictGroups)
	}

	ch1 <- &conflictInfo{
		conflictGroups: conflictGroups,
		conflictTxs:    conflictTxs,
	}
}

func overlapped(set0 map[int]struct{}, set1 map[int]struct{}) bool {
	for k, _ := range set0 {
		if _, ok := set1[k]; ok {
			return true
		}
	}
	return false
}

func (pb *ParallelBlock) executeGroup(group *ExecutionGroup, ch chan *txInfo) {
	var (
		gp      = new(GasPool).AddGas(pb.block.GasLimit())
		statedb = group.statedb
	)

	// Iterate over and process the individual txInfos
	for i := 0; i < len(group.txInfos); i++ {
		if i < group.startTrxIndex {
			ch <- group.txInfos[i]
			continue
		}
		feeAmount := big.NewInt(0)
		txInfo := group.txInfos[i]
		txHash := txInfo.hash
		ti := txInfo.index
		statedb.Prepare(txHash, pb.block.Hash(), ti)
		receipt, err := ApplyTransactionMsg(pb.config, pb.context, gp, statedb, pb.block.Header(),
			txInfo.msg, txInfo.tx, &group.usedGas, feeAmount, pb.vmConfig)

		trxUsedGas := receipt.GasUsed
		group.AddFeeAmount(feeAmount)
		if err != nil {
			group.err = err
			group.errTxIndex = ti
			txInfo.result = NewTrxResult(nil, statedb.FinalizeTouchedAddress(), trxUsedGas, feeAmount)
			group.SetStartTrxPos(-1)
			ch <- txInfo
			return
		}
		txInfo.result = NewTrxResult(receipt, statedb.FinalizeTouchedAddress(), trxUsedGas, feeAmount)
		ch <- txInfo
	}
	group.SetStartTrxPos(-1)
}

func (pb *ParallelBlock) execute(group *ExecutionGroup, ch chan *txInfo, chForUpdateCache chan *addressRlpDataPair, chForExit chan bool) {
	if group.startTrxIndex != -1 {
		pb.executeGroup(group, ch)
	} else {
		for _, txInfo := range group.txInfos {
			ch <- txInfo
		}
	}

	if len(pb.executionGroups) == 1 {
		pb.statedb.FinaliseGroup(true)
		return
	}
	stateObjsToReuse := make(map[common.Address]struct{})
	for _, txInfo := range group.txInfos {
		if result := txInfo.result; result != nil {
			appendStateObjToReuse(stateObjsToReuse, result.touchedAddresses)
		}
	}

	for addr := range stateObjsToReuse {
		select {
		case <-chForExit:
			return
		default:
			stateObj, data, changed := pb.statedb.CopyStateObjRlpDataFromOtherDB(group.statedb, addr)
			if changed {
				chForUpdateCache <- &addressRlpDataPair{
					address:  addr,
					stateObj: stateObj,
					rlpData:  data,
				}
			}
		}
	}
}

func (pb *ParallelBlock) executeInParallelAndCheckConflict() (types.Receipts, []*types.Log, uint64, error) {
	var (
		err                 error
		errIndex            = -1
		receipts            = make(types.Receipts, len(pb.txInfos))
		usedGas             = uint64(0)
		allLogs             []*types.Log
		gp                  = new(GasPool).AddGas(pb.block.GasLimit())
		cumulative          = uint64(0)
		wg                  = sync.WaitGroup{}
		chForAssociatedAddr = make(chan *txInfo, len(pb.txInfos))
		chForUpdateCache    = make(chan *addressRlpDataPair, len(pb.txInfos)*2)
		chForFinish         = make(chan *state.StateDB)
	)

	for {
		txInfoCh := make(chan *txInfo, len(pb.txInfos))
		conflictInfoCh := make(chan *conflictInfo)
		var chsForExist []chan bool

		if len(pb.executionGroups) != 1 {
			go pb.checkGroupsConflict(txInfoCh, conflictInfoCh)
		}
		go pb.updateStateDB(chForUpdateCache, chForFinish)
		go pb.processAssociatedAddressOfContract(chForAssociatedAddr)

		for _, group := range pb.executionGroups {
			wg.Add(1)
			chForExist := make(chan bool)
			chsForExist = append(chsForExist, chForExist)

			go func(group *ExecutionGroup, chForExit chan bool) {
				defer wg.Done()
				pb.execute(group, txInfoCh, chForUpdateCache, chForExit)
			}(group, chForExist)
		}

		if len(pb.executionGroups) != 1 {
			result := <-conflictInfoCh
			if len(result.conflictGroups) != 0 {
				for _, ch := range chsForExist {
					close(ch)
				}
				wg.Wait()
				close(chForUpdateCache)
				<-chForFinish

				pb.reGroupAndRevert(result.conflictGroups, result.conflictTxs)
				chForUpdateCache = make(chan *addressRlpDataPair, len(pb.txInfos)*2)
				continue
			}
		} else {
			wg.Wait()
		}
		break
	}

	for _, group := range pb.executionGroups {
		if group.err != nil && (group.errTxIndex < errIndex || errIndex == -1) {
			err = group.err
			errIndex = group.errTxIndex
		}
		usedGas += group.usedGas
		pb.feeAmount.Add(group.feeAmount, pb.feeAmount)

		// Update contract associated address
		for _, txInfo := range group.txInfos {
			if result := txInfo.result; result != nil {
				receipts[txInfo.index] = result.receipt
				chForAssociatedAddr <- txInfo
			}
		}
	}
	close(chForAssociatedAddr)

	for index, tx := range pb.block.Transactions() {
		if gasErr := gp.SubGas(tx.Gas()); gasErr != nil {
			return nil, nil, 0, gasErr
		}

		if errIndex != -1 && index >= errIndex {
			return nil, nil, 0, err
		}

		gp.AddGas(tx.Gas() - receipts[index].GasUsed)
		cumulative += receipts[index].GasUsed
		receipts[index].CumulativeGasUsed = cumulative
		allLogs = append(allLogs, receipts[index].Logs...)
	}
	wg.Wait()
	close(chForUpdateCache)
	db := <-chForFinish
	*pb.statedb = *db

	return receipts, allLogs, usedGas, nil
}

func (pb *ParallelBlock) prepare() error {
	txCount := pb.block.Transactions().Len()
	contractAddrs := make([]common.Address, 0, txCount)
	chForError := make(chan error, txCount)
	wg := sync.WaitGroup{}

	for ti, tx := range pb.block.Transactions() {
		wg.Add(1)
		go func(ti int, tx *types.Transaction) {
			msg, err := tx.AsMessage(types.MakeSigner(pb.config, pb.block.Header().Number))
			if err != nil {
				chForError <- err
				return
			}
			txInfo := newTxInfo(ti, tx.Hash(), &msg, tx)
			pb.txInfos[ti] = txInfo
			wg.Done()
		}(ti, tx)

		if to := tx.To(); to != nil {
			contractAddrs = append(contractAddrs, *to)
		}
	}

	pb.associatedAddressMap = associatedAddressMngr.LoadAssociatedAddresses(contractAddrs)
	wg.Wait()

	if len(chForError) != 0 {
		return <-chForError
	}

	return nil
}

func (pb *ParallelBlock) prepareAndGroup() error {
	txCount := pb.block.Transactions().Len()
	contractAddrs := make([]common.Address, 0, txCount)
	chForTxInfo := make(chan *txInfo, txCount)
	chForGroup := make(chan bool)

	go pb.group(chForTxInfo, chForGroup)

	for ti, tx := range pb.block.Transactions() {
		msg, err := tx.AsMessage(types.MakeSigner(pb.config, pb.block.Header().Number))
		if err != nil {
			close(chForTxInfo)
			return err
		}
		txInfo := newTxInfo(ti, tx.Hash(), &msg, tx)
		chForTxInfo <- txInfo
		pb.txInfos[ti] = txInfo
		if to := tx.To(); to != nil {
			contractAddrs = append(contractAddrs, *to)
		}
	}

	close(chForTxInfo)
	pb.associatedAddressMap = associatedAddressMngr.LoadAssociatedAddresses(contractAddrs)
	<-chForGroup

	return nil
}

func (pb *ParallelBlock) processAssociatedAddressOfContract(ch chan *txInfo) {
	associatedAddrs := make(map[common.Address]*state.TouchedAddressObject)

	for txInfo := range ch {
		if to := txInfo.tx.To(); to != nil {
			touchedAddr := txInfo.result.touchedAddresses.Copy()
			msg := txInfo.msg

			touchedAddr.RemoveAccount(msg.From())
			touchedAddr.RemoveAccount(msg.Payment())
			touchedAddr.RemoveAccountsInArgs()
			if len(touchedAddr.AccountOp()) > 1 {
				associatedAddrs[*to] = touchedAddr
			}
		}
	}

	associatedAddressMngr.UpdateAssociatedAddresses(associatedAddrs)
}

func (pb *ParallelBlock) updateStateDB(ch chan *addressRlpDataPair, chForFinish chan *state.StateDB) {
	if len(pb.executionGroups) != 1 {
		db := pb.statedb.Copy()
		for addrData := range ch {
			db.UpdateDBTrie(addrData.address, addrData.stateObj, addrData.rlpData)
		}
		chForFinish <- db
	} else {
		chForFinish <- pb.statedb
	}
}

func (pb *ParallelBlock) Process() (types.Receipts, []*types.Log, uint64, error) {
	if pb.block.Transactions().Len() == 0 {
		return nil, nil, 0, nil
	}
	if err := pb.prepareAndGroup(); err != nil {
		return nil, nil, 0, err
	}

	return pb.executeInParallelAndCheckConflict()
}

func sortTxInfosByIndex(txInfos []*txInfo) []*txInfo {
	sort.Slice(txInfos, func(i, j int) bool {
		return txInfos[i].index < txInfos[j].index
	})
	return txInfos
}
