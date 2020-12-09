package testlog

import "truechain/discovery/log"

var msg string = "P2P"

func AddLog(ctx ...interface{}) {
	log.Info(msg, ctx...)
}
