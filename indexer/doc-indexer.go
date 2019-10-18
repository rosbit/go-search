package indexer

import (
	"github.com/go-ego/riot/types"
	"github.com/rosbit/go-wget"
	"go-search/conf"
	"net/http"
	"strings"
	"fmt"
	"log"
	"io"
	"os"
)

// IndexJSON/IndexCSV/... 等从文件获取doc建索引的函数签名
type FnIndexReader func(string,io.ReadCloser,...string)([]string,error)

// 把一个doc添加到索引库
func IndexDoc(index string, doc map[string]interface{}) (docId string, err error) {
	if !running {
		return "", fmt.Errorf("the service is stopped")
	}

	idx, err := initIndexer(index)
	if err != nil {
		return "", fmt.Errorf("schema %s not found, please create schema first", index)
	}

	docId, err = idx.indexDoc(doc)
	if err != nil {
		return "", err
	}
	idx.flush()
	return docId, nil
}

// 把多个JSON(JSON数组)添加到索引库
func IndexJSON(index string, in io.ReadCloser, cb ...string) (docIds []string, err error) {
	return indexFromDocGenerator(index, in, fromJsonFile, cb...)
}

// 把csv中的一行作为doc添加到索引库
func IndexCSV(index string, in io.ReadCloser, cb ...string) (docIds []string, err error) {
	return indexFromDocGenerator(index, in, fromCsvFile, cb...)
}

// 把JSON Lines(每行一个JSON)添加到索引库
func IndexJSONLines(index string, in io.ReadCloser, cb ...string) (docIds []string, err error) {
	return indexFromDocGenerator(index, in, fromJsonLines, cb...)
}

//从文件获取doc做索引的统一流程，不同的文件类型需要实现一个fnReaderGenerator
func indexFromDocGenerator(index string, in io.ReadCloser, docGenerator fnReaderGenerator, cb ...string) (docIds []string, err error) {
	var idx *indexer
	var docChan <-chan Doc

	if !running {
		err = fmt.Errorf("the service is stopped")
		goto ERROR
	}

	idx, err = initIndexer(index)
	if err != nil {
		err = fmt.Errorf("schema %s not found, please create schema first", index)
		goto ERROR
	}

	docChan, err = docGenerator(in)
	if err != nil {
		goto ERROR
	}
	if len(cb) == 0 {
		defer in.Close()
		// no callback
		return idx.indexDocs(docChan), nil
	}

	// with callback
	go func() {
		defer os.Remove(cb[1])
		defer in.Close()
		idx.indexDocs(docChan, cb...)
	}()
	return nil, nil

ERROR:
	in.Close()
	if len(cb) > 0 {
		os.Remove(cb[1])
	}
	return
}

// 删除一个doc
func DeleteDoc(index string, docId interface{}) error {
	if !running {
		return fmt.Errorf("the service is stopped")
	}

	idx, err := initIndexer(index)
	if err != nil {
		return fmt.Errorf("schema %s not found, please create schema first", index)
	}
	idx.deleteDoc(fmt.Sprintf("%v", docId))
	idx.flush()
	return nil
}

// 删除多个doc
func DeleteDocs(index string, docIds []interface{}) error {
	if !running {
		return fmt.Errorf("the service is stopped")
	}

	idx, err := initIndexer(index)
	if err != nil {
		return fmt.Errorf("schema %s not found, please create schema first", index)
	}
	for _, docId := range docIds {
		idx.deleteDoc(fmt.Sprintf("%v", docId))
	}
	idx.flush()
	return nil
}

//索引中增加一个文档
func (idx *indexer) indexDoc(doc map[string]interface{}) (string, error) {
	storedDoc := StoredDoc{}
	tokens := []types.TokenData{}

	fm := idx.schema.FieldMap
	fields := idx.schema.Fields
	engine := idx.engine
	startLoc := 0
	pk := map[int]interface{}{}
	for fieldName, value := range doc {
		fieldIdx, ok := fm[fieldName]
		if !ok {
			continue
		}
		field := &fields[fieldIdx]

		val, err := field.ToNativeValue(value)
		if err != nil {
			return "", err
		}
		if field.PK {
			pk[fieldIdx] = val
		}

		switch val.(type) {
		case string:
			s := val.(string)
			var segTokens []string
			switch field.Tokenizer {
			case conf.ZH_TOKENIZER:
				// segTokens = engine.Segment(s)
				segTokens = hanziTokenize(s)
			case conf.NONE_TOKENIZER:
				// segTokens = []string{strings.TrimSpace(s)}
				val = strings.TrimSpace(s)
			default:
				segTokens = whitespaceTokenize(s)
			}
			if len(segTokens) > 0 {
				fieldTokens := buildIndexTokens(fieldIdx, segTokens, startLoc)
				tokens = append(tokens, fieldTokens...)
				startLoc += len(fieldTokens) + 10 // 与下一字段的索引间加上几个间隔
			}
		default:
		}

		storedDoc[fieldName] = val
	}
	pkIdx := idx.schema.PKIdx
	if len(pk) != len(pkIdx) {
		return "", fmt.Errorf("pk field must be specified")
	}

	docId := strings.Builder{}
	for i, idx := range pkIdx {
		if i > 0 {
			docId.WriteByte('_')
		}
		docId.WriteString(fmt.Sprintf("%v", pk[idx]))
	}

	dId := docId.String()
	count := mergeTokenLocs(&tokens)
	indexerChan <- &indexerOp{
		op: _INDEX_DOC,
		engine: engine,
		docId: dId,
		doc: &types.DocData{
			Tokens: tokens[:count],
			Fields: storedDoc,
			Labels: allDocs,
		},
	}
	return dId, nil
}

//批量增加索引文档
func (idx *indexer) indexDocs(docs <-chan Doc, cb ...string) (docIds []string) {
	hasError := false
	hasCb := len(cb) > 0

	count := 0
	for doc := range docs {
		if doc.err != nil {
			if !hasCb {
				docIds = append(docIds, doc.err.Error())
			} else {
				log.Printf("[error] indexing %s: %v\n", idx.schema.Name, doc.err.Error())
			}
			continue
		}

		if docId, err := idx.indexDoc(doc.doc); err != nil {
			if !hasCb {
				docIds = append(docIds, err.Error())
			} else {
				log.Printf("[error] indexing %s: %v\n", idx.schema.Name, err.Error())
			}
			hasError = true
		} else {
			if !hasCb {
				docIds = append(docIds, docId)
			}
			count += 1
		}
	}

	if count > 0 {
		idx.flush()
	}
	log.Printf("[info] %d docs appended to index %s\n", count, idx.schema.Name)

	if hasCb {
		params := func() map[string]interface{} {
			if hasError {
				return map[string]interface{}{
					"code": http.StatusInternalServerError,
					"msg": "failed to index docs",
					"index": idx.schema.Name,
					"docs": count,
				}
			}
			return map[string]interface{}{
				"code": http.StatusOK,
				"msg": "OK",
				"index": idx.schema.Name,
				"docs": count,
			}
		}

		status, content, _, err := wget.PostJson(cb[0], "POST", params(), nil)
		if err != nil {
			log.Printf("failed to send callback to %s: %d\n", cb[0], status)
		} else {
			log.Printf("send to callback to %s OK: %s\n", cb[0], string(content))
		}
	}
	return
}

//给每个token加上位置信息，同时生成某个字段内的索引
func buildIndexTokens(fieldIdx int, tokens []string, startLoc int) []types.TokenData {
	j := len(tokens)
	res := make([]types.TokenData, j*2)
	for i, token := range tokens {
		res[i] = types.TokenData{
			Text: token,
			Locations: []int{startLoc+i},
		}

		res[j] = types.TokenData{
			Text: fmt.Sprintf("f%d:%s", fieldIdx, token),
			Locations: []int{startLoc+j},
		}
		j += 1
	}

	return res
}

func mergeTokenLocs(pTokens *[]types.TokenData) int {
	tokens := *pTokens
	c := len(tokens)
	pos := make(map[string]int, c) // token -> idx in tokens
	count := 0

	for i:=0; i<c; i++ {
		token := &tokens[i]
		if idx, ok := pos[token.Text]; !ok {
			pos[token.Text] = count
			if count != i {
				tokens[count] = *token
			}
			count += 1
		} else {
			mToken := &tokens[idx]
			mToken.Locations = append(mToken.Locations, token.Locations...)
		}
	}
	return count
}

func (idx *indexer) deleteDoc(docId string) {
	indexerChan <- &indexerOp{
		op:     _DELETE_DOC,
		engine: idx.engine,
		docId:  docId,
	}
}

func (idx *indexer) flush() {
	indexerChan <- &indexerOp{
		op:     _FLUSH_DOC,
		engine: idx.engine,
	}
}
