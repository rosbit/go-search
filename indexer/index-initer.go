package indexer

import (
	"github.com/go-ego/riot"
	"github.com/go-ego/riot/types"
	"go-search/conf"
	"fmt"
	"log"
	"encoding/gob"
	"time"
)

// 初始化/获取索引库
func initIndexer(index string) (*indexer, error) {
	indexerLock.RLock()
	idx, ok := indexers[index]
	indexerLock.RUnlock()

	if ok {
		log.Printf("[LRU] index %s (existing) added to LRU\n", index)
		lruAdd(index)
		return idx, nil
	}

	schema, err := conf.LoadSchema(index)
	if err != nil {
		return nil, fmt.Errorf("schema of %s not found, please create schema first", index)
	}

	gob.Register(StoredDoc{})
	engine := &riot.Engine{}
	idx = &indexer{schema:schema, engine:engine}
	initOpts := types.EngineOpts{
		UseStore:    len(conf.UseStore) > 0,
		NotUseGse:   true,
		/*
		StoreEngine: "bg",
		StoreFolder: schema.StorePath,
		NumShards:   int(schema.Shards),
		*/
		//IndexerOpts: &types.IndexerOpts{
		//	IndexType: types.LocsIndex,
		//},
	}
	if len(conf.UseStore) > 0 {
		initOpts.StoreEngine = conf.UseStore
		initOpts.StoreFolder = schema.StorePath
		initOpts.NumShards   = int(schema.Shards)
	}
	//if schema.NeedZhSeg {
	//	segDict := &conf.ServiceConf.SegDict
	//	initOpts.Using = 3
	//	initOpts.GseDict = segDict.DictFile
	//	initOpts.StopTokenFile = segDict.StopFile
	//} else {
	//	initOpts.NotUseGse = true
	//}
	engine.Init(initOpts)
	engine.Flush()
	log.Printf("[LRU] index %s (new) added to LRU\n", index)
	lruAdd(index)

	indexerLock.Lock()
	defer indexerLock.Unlock()

	indexers[index] = idx
	return idx, nil
}

func RemoveIndexer(index string) {
	indexerLock.RLock()
	idx, ok := indexers[index]
	indexerLock.RUnlock()

	if !ok {
		return
	}

	indexerLock.Lock()
	delete(indexers, index)
	indexerLock.Unlock()

	go func() {
		idx.engine.Close()
	}()
}

// -------------------------------------

const (
	_INDEX_DOC = iota
	_DELETE_DOC
	_FLUSH_DOC
)

type indexerOp struct {
	op      int
	engine *riot.Engine
	docId   string
	doc    *types.DocData
}

var (
	indexerChan chan *indexerOp
	stopChan chan struct{}
	running bool
	lruTicker *time.Ticker
)

func StartIndexers(workNum int) {
	indexerChan = make(chan *indexerOp, workNum)
	stopChan = make(chan struct{})
	running = true
	for i:=0; i<workNum; i++ {
		go opThread(i)
	}

	lruTicker = time.NewTicker(time.Duration(conf.ServiceConf.LruMinutes) * time.Minute)
	go lruThread()
}

func IsRunning() bool {
	return running
}

func StopIndexers(workNum int) {
	if !running {
		return
	}

	running = false
	close(indexerChan)
	lruTicker.Stop()

	for i:=0; i<workNum; i++ {
		<-stopChan
	}

	for name, idx := range indexers {
		log.Printf("stopping index %s ...\n", name)
		idx.engine.Close()
	}
}

func opThread(workNo int) {
	for opData := range indexerChan {
		op, engine, docId, doc := opData.op, opData.engine, opData.docId, opData.doc
		switch op {
		case _INDEX_DOC:
			engine.IndexDoc(docId, *doc, true)
		case _DELETE_DOC:
			engine.RemoveDoc(docId, true)
		case _FLUSH_DOC:
			engine.Flush()
		}
	}

	stopChan <-struct{}{}
}

func lruThread() {
	timespan := time.Duration(conf.ServiceConf.LruMinutes) * time.Minute
	for now := range lruTicker.C {
		indexes := lruGet(now.Add(-timespan))
		for index := range indexes {
			RemoveIndexer(index)
			log.Printf("[LRU] index %s will be closed\n", index)
		}
	}
}
