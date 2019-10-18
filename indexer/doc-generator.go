package indexer

import (
	"io"
	"encoding/csv"
	"encoding/json"
)

type Doc struct {
	doc map[string]interface{}
	err error
}

//从reader依次获取doc的函数签名
type fnReaderGenerator func(io.Reader) (<-chan Doc, error)

//从doc数组依次获取doc
func fromArray(docs []map[string]interface{}) (<-chan Doc, error) {
	docChan := make(chan Doc)

	go func() {
		for _, doc := range docs {
			docChan <- Doc{doc, nil}
		}
		close(docChan)
	}()
	return docChan, nil
}

//从JSON数组文件依次获取doc
func fromJsonFile(in io.Reader) (<-chan Doc, error) {
	dec := json.NewDecoder(in)
	var docs []map[string]interface{}
	if err := dec.Decode(&docs); err != nil {
		return nil, err
	}
	return fromArray(docs)
}

//从csv文件依次读取doc，第一行是标题
func fromCsvFile(in io.Reader) (<-chan Doc, error) {
	docChan := make(chan Doc)

	csvIn := csv.NewReader(in)
	fields, err := csvIn.Read()
	if err != nil {
		close(docChan)
		if err == io.EOF {
			return docChan, nil
		}

		return nil, err
	}

	go func() {
		l := len(fields)

		for {
			rec, err := csvIn.Read()
			if err != nil {
				if err == io.EOF {
					close(docChan)
					break
				}
				docChan <- Doc{nil, err}
				continue
			}

			doc := make(map[string]interface{}, l)
			for i, field := range fields {
				doc[field] = rec[i]
			}
			docChan <- Doc{doc, nil}
		}
	}()

	return docChan, nil
}

//从JSON Lines文件(每行一个JSON)依次读取doc；JSON间不能有','
func fromJsonLines(in io.Reader) (<-chan Doc, error) {
	docChan := make(chan Doc)

	go func() {
		dec := json.NewDecoder(in)
		for dec.More() {
			var doc map[string]interface{}
			if err := dec.Decode(&doc); err != nil {
				break
			}
			docChan <- Doc{doc, nil}
		}

		close(docChan)
	}()

	return docChan, nil
}

