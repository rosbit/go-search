package indexer

import (
	"github.com/go-ego/riot/types"
	"strings"
	"fmt"
)

// 根据已有字段，组装成docid的过滤条件
func (idx *indexer) makeDocIdFilters(doc map[string]interface{}) (filters string, err error) {
	pkIdx := idx.schema.PKIdx
	fm := idx.schema.FieldMap
	fields := idx.schema.Fields

	pkCount := len(pkIdx)
	pk := make([]string, pkCount)
	c := 0

	for fieldName, value := range doc {
		fieldIdx, ok := fm[fieldName]
		if !ok {
			continue
		}
		field := &fields[fieldIdx]

		if field.PK {
			pk[c] = fmt.Sprintf("%s:%v", fieldName, value)
			c += 1
			if c >= pkCount {
				break
			}
		}
	}
	if c < pkCount {
		return "", fmt.Errorf("pk number not matched")
	}
	return strings.Join(pk, ";"), nil
}

func (idx *indexer) getDoc(doc map[string]interface{}) (map[string]interface{}, error) {
	filters, err := idx.makeDocIdFilters(doc)
	if err != nil {
		return nil, err
	}

	pq, err := parseQuery("", "", "", filters, "1", "1", "")
	if err != nil {
		return nil, err
	}
	sr, err := idx.pq2SearchQuery(pq)
	if err != nil {
		return nil, err
	}
	searchResp := idx.engine.Search(*sr)

	if searchResp.Docs == nil {
		return nil, nil
	}
	docs, ok := searchResp.Docs.(types.ScoredDocs)
	if !ok || len(docs) == 0 {
		return nil, nil
	}

	schema := idx.schema
	var retDoc StoredDoc
	for _, doc := range docs {
		storedDoc, ok := doc.Fields.(StoredDoc)
		if !ok {
			continue
		}
		retDoc = StoredDoc{}
		for k, v := range storedDoc {
			if fIdx, ok := schema.TimeIdx[k]; !ok {
				retDoc[k] = v
			} else {
				field := &schema.Fields[fIdx]
				retDoc[k] = field.FormatDatetime(v)
			}
		}
		return retDoc, nil
	}
	return nil, nil
}
