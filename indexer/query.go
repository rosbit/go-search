package indexer

import (
	"github.com/go-ego/riot/types"
	"go-search/conf"
	"fmt"
	"reflect"
	"strings"
)

// 根据参数完成实际的搜索查询
func Query(index, q, fq, s, f, page, pagesize, fl string) (pagination interface{}, timeout bool, docs <-chan interface{}, err error) {
	if !running {
		return nil, false, nil, fmt.Errorf("the service is stopped")
	}

	pq, err := parseQuery(q, fq, s, f, page, pagesize, fl)
	if err != nil {
		return nil, false, nil, err
	}

	idx, err := initIndexer(index)
	if err != nil {
		return nil, false, nil, err
	}

	sr, err := idx.pq2SearchQuery(pq)
	if err != nil {
		return nil, false, nil, err
	}
	fmt.Printf("pq: %#v\n", pq)
	fmt.Printf("sr: %v\n", *sr)

	resp := idx.engine.Search(*sr)
	pagination, timeout, docs = idx.outputResult(&resp, pq)
	return
}

// 转换为搜索引擎的搜索参数
func (idx *indexer) pq2SearchQuery(pq *parsedQuery) (*types.SearchReq, error) {
	// fl
	fm := idx.schema.FieldMap
	if pq.outFieldList != nil && len(pq.outFieldList) > 0 {
		for _, fn := range pq.outFieldList {
			if _, ok := fm[fn]; !ok {
				return nil, fmt.Errorf("out field %s not found", fn)
			}
		}
	}

	sr := types.SearchReq{
		RankOpts: &types.RankOpts{
			ScoringCriteria: &scorerT{
				schema: idx.schema,
				pq: pq,
			},
			OutputOffset: pq.start,
			MaxOutputs: pq.rows,
		},
	}

	if pq.labels == nil {
		sr.Logic = types.Logic{
			Expr: types.Expr{
				Must:   []string{},
				Should: []string{},
				NotIn:  []string{},
			},
		}
		// q
		idx.generateTokens(pq.should, &sr.Logic.Should, &sr.Logic.Expr.Should)
		idx.generateTokens(pq.must, &sr.Logic.Must, &sr.Logic.Expr.Must)
		idx.generateTokens(pq.notIn, &sr.Logic.NotIn, &sr.Logic.Expr.NotIn)
	}

	// fq
	if pq.fquerys != nil && len(pq.fquerys) > 0 {
		for _, fq := range pq.fquerys {
			fIdx, ok := fm[fq.fieldName]
			if !ok {
				continue
			}

			idx.generateFieldTokens(fIdx, fq.query.should, &sr.Logic.Should, &sr.Logic.Expr.Should)
			idx.generateFieldTokens(fIdx, fq.query.must, &sr.Logic.Must, &sr.Logic.Expr.Must)
			idx.generateFieldTokens(fIdx, fq.query.notIn, &sr.Logic.NotIn, &sr.Logic.Expr.NotIn)
		}
	}

	// if there's not, there's must
	if sr.Logic.NotIn && !sr.Logic.Must {
		sr.Logic.Must = true
		sr.Logic.Expr.Must = allDocs
	}
	// if there's no query, it means query all result
	if !(sr.Logic.Must || sr.Logic.Should) {
		sr.Labels = allDocs
		sr.Tokens = allDocs
	}

	// s
	checkSortings(&pq.sortBys, fm)
	if pq.sortBys == nil {
		pq.sortBys = makeDefaultSortBys(idx.schema)
	}

	// f
	checkFilters(&pq.filters, idx.schema)

	return &sr, nil
}

func (idx *indexer) generateTokens(qs []string, flag *bool, res *[]string) {
	if qs == nil || len(qs) == 0 {
		return
	}
	for _, q := range qs {
		// *res = append(*res, idx.engine.Segment(q)...)
		*res = append(*res, hanziTokenize(q)...)
	}
	if len(*res) > 0 {
		*flag = true
	}
}

func (idx *indexer) generateFieldTokens(fIdx int, qs []string, flag *bool, res *[]string) {
	if qs == nil || len(qs) == 0 {
		return
	}

	tokenizer := idx.schema.Fields[fIdx].Tokenizer
	c := 0
	for _, q := range qs {
		var tokens []string
		switch tokenizer {
		case conf.ZH_TOKENIZER:
			// tokens = idx.engine.Segment(q)
			tokens = hanziTokenize(q)
		case conf.NONE_TOKENIZER:
			// tokens = []string{strings.TrimSpace(q)}
		default:
			tokens = whitespaceTokenize(q)
		}
		c += len(tokens)
		for _, t := range tokens {
			*res = append(*res, fmt.Sprintf("f%d:%s", fIdx, t))
		}
	}
	if c > 0 {
		*flag = true
	}
}

func checkSortings(pqSortBys *[]sorting, fm map[string]int) {
	sortBys := *pqSortBys
	if sortBys == nil || len(sortBys) == 0 {
		*pqSortBys = nil
		return
	}

	count := 0
	c := len(sortBys)
	for i:=0; i<c; i++ {
		s := &sortBys[i]
		if fIdx, ok := fm[s.fieldName]; !ok {
			continue
		} else {
			s.fIdx = fIdx
		}

		if count != i {
			sortBys[count] = *s
		}
		count += 1
	}

	if count <= 0 {
		*pqSortBys = nil
		return
	}

	if count < c {
		*pqSortBys = sortBys[:count]
	}
}

func makeDefaultSortBys(schema *conf.Schema) []sorting {
	if schema.DefSortBys != nil && len(schema.DefSortBys) > 0 {
		sortBys := make([]sorting, len(schema.DefSortBys))
		for i, sortBy := range schema.DefSortBys {
			sortBys[i] = sorting{
				fieldName: schema.Fields[sortBy.FieldIdx].Name,
				asc:       sortBy.Ascending,
				fIdx:      sortBy.FieldIdx,
			}
		}
		return sortBys
	}

	sortBys := make([]sorting, len(schema.PKIdx))
	for i, fIdx := range schema.PKIdx {
		sortBys[i] = sorting{
			fieldName: schema.Fields[fIdx].Name,
			asc:       true, // always true
			fIdx:      fIdx,
		}
	}
	return sortBys
}

func checkFilters(pqFilters *[]filter, schema *conf.Schema) {
	filters := *pqFilters
	if filters == nil || len(filters) == 0 {
		*pqFilters = nil
		return
	}

	count := 0
	c := len(filters)
	for i:=0; i<c; i++ {
		f := &filters[i]
		if fIdx, ok := schema.FieldMap[f.fieldName]; !ok {
			continue
		} else {
			f.fIdx = fIdx
		}
		fieldConf := &schema.Fields[f.fIdx]

		// conds
		checkFilterConds(fieldConf, &f.conds)
		// ranges
		checkFilterRanges(fieldConf, &f.ranges)

		if f.conds == nil && f.ranges == nil {
			continue
		}

		if count != i {
			filters[count] = *f
		}
		count += 1
	}

	if count <= 0 {
		*pqFilters = nil
		return
	}
	if count < c {
		*pqFilters = filters[:count]
	}
}

func checkFilterConds(field *conf.Field, fconds *[]interface{}) {
	conds := *fconds
	if conds == nil || len(conds) == 0 {
		*fconds = nil
		return
	}

	c := len(conds)
	count := 0
	for i:=0; i<c; i++ {
		v, err := field.ToNativeValue(conds[i])
		if err != nil {
			continue
		}
		conds[count] = v
		count += 1
	}

	if count <= 0 {
		*fconds = nil
		return
	}

	if count < c {
		*fconds = conds[:count]
	}
}

func checkFilterRanges(field *conf.Field, franges *[]range_) {
	ranges := *franges
	if ranges == nil || len(ranges) == 0 {
		*franges = nil
		return
	}

	c := len(ranges)
	count := 0
	for i:=0; i<c; i++ {
		r := &ranges[i]
		if r.from.(string) == "" {
			r.from = nil
		} else {
			if v, err := field.ToNativeValue(r.from); err != nil {
				continue
			} else {
				r.from = v
			}
		}

		if r.to.(string) == "" {
			r.to = nil
		} else {
			if v, err := field.ToNativeValue(r.to); err != nil {
				continue
			} else {
				r.to = v
			}
		}

		if r.from == nil && r.to == nil {
			continue
		}

		ranges[count] = *r
		count += 1
	}

	if count <= 0 {
		*franges = nil
		return
	}
	if count < c {
		*franges = ranges[:count]
	}
}

// 打分需要的数据，必须实现types.ScoringCriteria
type scorerT struct {
	schema *conf.Schema
	pq     *parsedQuery
}

const (
	// 最大临近距离
	MaxTokenProximity = 2
)

// 打分函数，是types.ScoringCriteria接口定义的函数
func (scorer *scorerT) Score(doc types.IndexedDoc, fields interface{}) []float32 {
	// 评分第一步
	// fmt.Printf("doc.TokenProximity: %d\n", doc.TokenProximity)
	if doc.TokenProximity > MaxTokenProximity {
		return []float32{}
	}

	if reflect.TypeOf(fields) != reflect.TypeOf(StoredDoc{}) {
		return []float32{}
	}
	storedDoc := fields.(StoredDoc)

	// 通过字段过滤去掉不需要的doc
	if !storedDoc.satisfied(scorer.pq.filters, scorer.schema) {
		return []float32{}
	}

	// fmt.Printf("doc.BM25: %v\n", doc.BM25)
	/*
	if scorer.pq.sortBys == nil {
		// fmt.Printf("doc.BM25: %v\n", doc.BM25)
		return []float32{float32(int(doc.BM25))}
	}*/

	return storedDoc.score(scorer.pq.sortBys, int(doc.BM25))
}

func (d StoredDoc) score(sortBys []sorting, bm25 int) []float32 {
	output := make([]float32, len(sortBys))
	for i, sortBy := range sortBys {
		// fIdx := sortBy.fIdx
		storedVal, ok := d[sortBy.fieldName]
		if !ok || storedVal == nil {
			output[i] = 0
			continue
		}

		output[i] = sortingScore(storedVal, bm25)
		if sortBy.asc && output[i] != 0 {
			output[i] = float32(1.0)/output[i]
		}
	}
	return output
}

func sortingScore(storedVal interface{}, bm25 int) float32 {
	v := reflect.ValueOf(storedVal)

	switch storedVal.(type) {
	case string:
		return float32(bm25)
	case int8, int16, int32, int64, int:
		return float32(v.Int())
	case uint8, uint16, uint32, uint64, uint:
		return float32(v.Uint())
	case float32, float64:
		return float32(v.Float())
	case bool:
		b := storedVal.(bool)
		if b {
			return float32(2)
		}
		return float32(1)
	default:
		return float32(0)
	}
}

func (d StoredDoc) satisfied(filters []filter, schema *conf.Schema) bool {
	if filters == nil {
		return true
	}

	for _, f := range filters {
		// fIdx := f.fIdx
		storedVal, ok := d[f.fieldName]
		if !ok || storedVal == nil {
			return false
		}

		if f.conds != nil {
			found := false
			tokenizer := schema.Fields[f.fIdx].Tokenizer
			for _, cond := range f.conds {
				if condEquals(storedVal, cond, tokenizer) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}

		if f.ranges != nil {
			found := false
			for _, r := range f.ranges {
				if inRange(storedVal, &r) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	return true
}

func condEquals(storedVal, cond interface{}, tokenizer string) bool {
	switch cond.(type) {
	case string:
		cv := cond.(string)
		sv := storedVal.(string)
		switch tokenizer {
		case conf.ZH_TOKENIZER:
			return strings.Contains(sv,  cv)
		case conf.NONE_TOKENIZER:
			return cv == strings.TrimSpace(sv)
		default:
			tokens := whitespaceTokenize(sv)
			for _, token := range tokens {
				if cv == token {
					return true
				}
			}
			return false
		}
	default:
		return storedVal == cond
	}
}

func inRange(storedVal interface{}, r *range_) bool {
	switch storedVal.(type) {
	case string:
		sv := storedVal.(string)
		if r.from != nil {
			r1 := r.from.(string)
			if sv < r1 {
				return false
			}
		}
		if r.to != nil {
			r2 := r.to.(string)
			if sv > r2 {
				return false
			}
		}
	case int8, int16, int32, int64, int:
		sv := reflect.ValueOf(storedVal).Int()
		if r.from != nil {
			r1 := reflect.ValueOf(r.from).Int()
			if sv < r1 {
				return false
			}
		}
		if r.to != nil {
			r2 := reflect.ValueOf(r.to).Int()
			if sv > r2 {
				return false
			}
		}
	case uint8, uint16, uint32, uint64, uint:
		sv := reflect.ValueOf(storedVal).Uint()
		if r.from != nil {
			r1 := reflect.ValueOf(r.from).Uint()
			if sv < r1 {
				return false
			}
		}
		if r.to != nil {
			r2 := reflect.ValueOf(r.to).Uint()
			if sv > r2 {
				return false
			}
		}
	case float32, float64:
		sv := reflect.ValueOf(storedVal).Float()
		if r.from != nil {
			r1 := reflect.ValueOf(r.from).Float()
			if sv < r1 {
				return false
			}
		}
		if r.to != nil {
			r2 := reflect.ValueOf(r.to).Float()
			if sv > r2 {
				return false
			}
		}
	default:
		return false
	}

	return true
}

func (idx *indexer) outputResult(searchResp *types.SearchResp, pq *parsedQuery) (pagination interface{}, timeout bool, docsCh chan interface{}) {
	p := struct {
		Total     int `json:"total"`
		Pages     int `json:"pages"`
		PageSize  int `json:"page-size"`
		CurrPage  int `json:"curr-page"`
		PageCount int `json:"page-count"`
	}{
		Total:    searchResp.NumDocs,
		Pages:    (searchResp.NumDocs + pq.rows - 1) / pq.rows,
		CurrPage: pq.start/pq.rows + 1,
		PageSize: pq.rows,
	}
	timeout = searchResp.Timeout
	pagination = &p

	if searchResp.Docs == nil {
		return
	}

	docs, ok := searchResp.Docs.(types.ScoredDocs)
	if !ok || len(docs) == 0 {
		return
	}
	// fmt.Printf("docs: %#v\n", docs)

	schema := idx.schema
	count := len(docs)
	p.PageCount = count
	docsCh = make(chan interface{})

	go func() {
		var retDoc StoredDoc

		for _, doc := range docs {
			storedDoc, ok := doc.Fields.(StoredDoc)
			if !ok {
				continue
			}

			outFieldList := pq.outFieldList
			if outFieldList == nil {
				if schema.TimeIdx == nil {
					retDoc = storedDoc
				} else {
					retDoc = StoredDoc{}
					for k, v := range storedDoc {
						if fIdx, ok := schema.TimeIdx[k]; !ok {
							retDoc[k] = v
						} else {
							field := &schema.Fields[fIdx]
							retDoc[k] = field.FormatDatetime(v)
						}
					}
				}
			} else {
				retDoc = StoredDoc{}
				for _, f := range outFieldList {
					if v, ok := storedDoc[f]; ok {
						if fIdx, ok := schema.TimeIdx[f]; !ok {
							retDoc[f] = v
						} else {
							field := &schema.Fields[fIdx]
							retDoc[f] = field.FormatDatetime(v)
						}
					}
				}
			}

			docsCh <- retDoc
		}

		close(docsCh)
	}()

	return
}
