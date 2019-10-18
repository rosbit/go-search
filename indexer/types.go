package indexer

import (
	"go-search/conf"
	"github.com/go-ego/riot"
	"sync"
)

// 索引库: 一个索引schema定义 + 一个搜索引擎实例
type indexer struct {
	schema *conf.Schema
	engine *riot.Engine
}

// q
type query struct {
	should  []string
	must    []string
	notIn   []string
}

// s
type sorting struct {
	fieldName string
	asc       bool
	fIdx      int  // set when querying
}

// filter range
type range_ struct {
	from interface{}
	to   interface{}
}

// f
type filter struct {
	fieldName string
	conds     []interface{}
	ranges    []range_
	fIdx      int // set when querying
}

type fquery struct {
	fieldName string
	*query
}

type parsedQuery struct {
	*query
	labels       []string
	fquerys      []fquery
	sortBys      []sorting
	filters      []filter
	start        int
	rows         int
	outFieldList []string
}

// 保存的字段，既用于显示，又用于过滤、打分
type StoredDoc map[string]interface{} // field name -> value

var (
	indexers    = map[string]*indexer{}  // index name => index
	indexerLock = &sync.RWMutex{}
	allDocs     = []string{"."}  // a tricky, 在没有q的情况下搜索所有的doc
)
