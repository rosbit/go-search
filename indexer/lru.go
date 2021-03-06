package indexer

import (
	"github.com/hashicorp/golang-lru"
	"go-search/conf"
	"time"
	"sync"
	"log"
)

var (
	lruAccess *lru.Cache
	tooOldLock = &sync.Mutex{}
	tooOldIndex = map[string]time.Time{}
)

func onEvict(key, value interface{}) {
	if conf.ServiceConf.LruMinutes <= 0 {
		return
	}
	tooOldLock.Lock()
	tooOldIndex[key.(string)] = value.(time.Time)
	tooOldLock.Unlock()
}

func init() {
	if conf.ServiceConf.LruMinutes <= 0 {
		return
	}
	c, err := lru.NewWithEvict(20, onEvict)
	if err != nil {
		log.Fatal("[LRU] failed to init LRU\n")
		return
	}
	lruAccess = c
}

func lruAdd(index string) {
	if conf.ServiceConf.LruMinutes <= 0 {
		return
	}
	tooOldLock.Lock()
	delete(tooOldIndex, index)
	tooOldLock.Unlock()

	lruAccess.Add(index, time.Now())
}

// used for renmaing
func LruRemove(index string) {
	if conf.ServiceConf.LruMinutes <= 0 {
		return
	}
	lruAccess.Remove(index)

	tooOldLock.Lock()
	delete(tooOldIndex, index)
	tooOldLock.Unlock()
}

func lruGet(timeLimit time.Time) (chan string) {
	if conf.ServiceConf.LruMinutes <= 0 {
		return nil
	}
	res := make(chan string)

	go func() {
		defer close(res)

		tooOldLock.Lock()
		indexes := make([]string, len(tooOldIndex))
		count := 0
		for k, v := range tooOldIndex {
			if v.Before(timeLimit) {
				indexes[count] = k
				count += 1
			}
		}
		if count > 0 {
			for _, index := range indexes[:count] {
				delete(tooOldIndex, index)
				res <- index
			}
		}
		tooOldLock.Unlock()

		k, v, ok := lruAccess.GetOldest()
		if !ok {
			return
		}
		index := k.(string)
		t := v.(time.Time)
		if t.Before(timeLimit) {
			lruAccess.Remove(k) // the key will go into tooOldIndex

			tooOldLock.Lock()
			defer tooOldLock.Unlock()
			delete(tooOldIndex, index)
			res <- index
		}
	}()

	return res
}

