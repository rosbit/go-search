package indexer

import (
	"github.com/go-ego/riot"
	"github.com/go-ego/riot/types"
	"go-search/conf"
	"fmt"
	"log"
	"encoding/gob"
)

// 初始化/获取索引库
func initIndexer(index string) (*indexer, error) {
	indexerLock.RLock()
	idx, ok := indexers[index]
	indexerLock.RUnlock()

	if ok {
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
		UseStore:    conf.UseStore,
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
	if conf.UseStore {
		initOpts.StoreEngine = "bg"
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
)

func StartIndexers(workNum int) {
	indexerChan = make(chan *indexerOp, workNum)
	stopChan = make(chan struct{})
	running = true
	for i:=0; i<workNum; i++ {
		go opThread(i)
	}
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

