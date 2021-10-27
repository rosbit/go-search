package rest

import (
	"github.com/rosbit/mgin"
	"go-search/indexer"
	"net/http"
)

// PUT /doc/:index
//
// add one document to index
//
// POST body:
// {
//   "field-name": "xxx",
//   ...
// }
func IndexDoc(c *mgin.Context) {
	updateDoc(c, indexer.IndexDoc, "doc added to index")
}

// PUT /update/:index
//
// update an existing document. there must be pk fields in the body.
//
// POST body:
// {
//   "field-name": "xxx",
//   ...
// }
func UpdateDoc(c *mgin.Context) {
	updateDoc(c, indexer.UpdateDoc, "doc updated to index")
}

func updateDoc(c *mgin.Context, fnUpdateDoc indexer.FnUpdateDoc, okStr string) {
	index := c.Param("index")

	var doc map[string]interface{}
	if code, err := c.ReadJSON(&doc); err != nil {
		c.Error(code, err.Error())
		return
	}
	docId, err := fnUpdateDoc(index, doc)
	if err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, map[string]interface{}{
		"code": http.StatusOK,
		"msg": okStr,
		"id": docId,
	})
}

// PUT /docs/:index[?cb=url-encoded-callback-url]
//
// add 1 or more documents to index
//
// path parameter
//  - index  name of index
// POST Head:
//   - Content-Type: multipart/form-data
//   arguments:
//   - file  file name with ext ".json"/".csv" to upload
//
// ---- OR ----
//
//   - Content-Type: application/json
//   POST body:
//   [
//     {doc 1},
//     {doc 2},
//      ...
//   ]
//
// ---- OR -----
//   - Content-Type: text/csv
//   POST body:
//   field-name1,fn2,fn3,...
//   val1,v2,v3,...
//   val1,v2,v3,...
//
// ---- OR -----
//   - Content-Type: application/x-ndjson
//   POST body:
//   {json}
//   {json}
func IndexDocs(c *mgin.Context) {
	index := c.Param("index")

	in, contentType, ext, err := getReader(c, "file")
	if err != nil {
		c.Error(http.StatusNotAcceptable, err.Error())
		return
	}
	defer in.Close()

	var indexReader indexer.FnIndexReader
	var ok bool
	if contentType == MULTIPART_FORM {
		if indexReader, ok = ext2Indexer[ext]; !ok {
			indexReader = indexer.IndexJSON
		}
	} else {
		if indexReader, ok = contentType2Indexer[contentType]; !ok {
			indexReader = indexer.IndexJSON
		}
	}

	cb := c.QueryParam("cb")
	if cb == "" {
		docIds, err := indexReader(index, in)
		if err != nil && docIds != nil {
			c.Error(http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, map[string]interface{}{
			"code": http.StatusOK,
			"msg": "docs added to index",
			"ids": docIds,
		})
	} else {
		tmpName, inTmp, err := saveTmpFile(in)
		if err != nil {
			c.Error(http.StatusInternalServerError, err.Error())
			return
		}
		indexReader(index, inTmp, cb, tmpName)
		c.JSON(http.StatusOK, map[string]interface{}{
			"code": http.StatusOK,
			"msg": "indexing request accepted",
		})
	}
}

var ext2Indexer = map[string]indexer.FnIndexReader{
	".csv":   indexer.IndexCSV,
	".jsonl": indexer.IndexJSONLines,
	".json":  indexer.IndexJSON,
}

var contentType2Indexer = map[string]indexer.FnIndexReader{
	JSON_MIME:      indexer.IndexJSON,
	CSV_MIME:       indexer.IndexCSV,
	JSONLINES_MIME: indexer.IndexJSONLines,
}
