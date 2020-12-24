package core

import (
	lru "github.com/hashicorp/golang-lru"
	"google.golang.org/genproto/googleapis/spanner/admin/database/v1"
	"truechain/discovery/common"
	"truechain/discovery/core/state"
)

const (
	associatedAddressCacheLimit = 10240
)

type AssociatedAddressMngr struct {
	lruCache *lru.Cache
}

func NewAssociatedAddressMngr() *AssociatedAddressMngr {
	lruCache, _ := lru.New(associatedAddressCacheLimit)

	return &AssociatedAddressMngr{
		lruCache: lruCache,
	}
}

func (aam *AssociatedAddressMngr) LoadAssociatedAddresses(addrs []common.Address) map[common.Address]*state.TouchedAddressObject {
	result := make(map[common.Address]*state.TouchedAddressObject)

	for _, addr := range addrs {
		if obj, exist := aam.lruCache.Get(addr); exist {
			result[addr] = obj.(*state.TouchedAddressObject)
		}
	}

	return result
}

func (aam *AssociatedAddressMngr) UpdateAssociatedAddresses(associatedAddrs map[common.Address]*state.TouchedAddressObject) {
	for addr, associatedAddr := range associatedAddrs {
		// remove accounts which come from args
		associatedAddr.RemoveAccountsInArgs()

		if obj, exist := aam.lruCache.Get(addr); !exist {
			aam.lruCache.Add(addr, associatedAddr)
		} else {
			touchedAddressObj := obj.(*state.TouchedAddressObject)
			touchedAddressObj.Merge(associatedAddr)
		}
	}
}
