package gorazor

import "sync"

//For gorazor just process on single one gohtml file now
//we use an singleton map to keep layout relationship
//Not a good solution but works
type LayManager struct {
	layOutMap  map[string][]Param
	fileLayOut map[string]string
}

var single *LayManager = nil
var mutexLock sync.RWMutex

func LayOutArgs(file string) []Param {
	mutexLock.RLock()
	defer mutexLock.RUnlock()
	manager := newManager()
	if args, ok := manager.layOutMap[file]; ok {
		return args
	}
	return []Param{}
}

func SetLayout(file string, args []Param) {
	mutexLock.Lock()
	manager := newManager()
	manager.layOutMap[file] = args
	mutexLock.Unlock()
}

func newManager() *LayManager {
	if single != nil {
		return single
	}
	lay := &LayManager{}
	lay.layOutMap = map[string][]Param{}
	lay.fileLayOut = map[string]string{}
	single = lay
	return lay
}
