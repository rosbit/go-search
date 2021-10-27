package rest

import (
	"github.com/rosbit/mgin"
	"net/http"
	"go-search/indexer"
)

// DELETE /doc/:index
//
// POST body:
// {
// 	  "id": "string"|integer|other-type,
// }
func DeleteDoc(c *mgin.Context) {
	index := c.Param("index")
	var doc struct {
		Id interface{} `json:"id"`
	}
	if code, err := c.ReadJSON(&doc); err != nil {
		c.Error(code, err.Error())
		return
	}
	if err := indexer.DeleteDoc(index, doc.Id); err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"code": http.StatusOK,
		"msg": "doc removed from index",
		"id":   doc.Id,
	})
}

// DELETE /docs/:index
//
// POST body:
// [
// 	  docId1, docId2, ...
// ]
func DeleteDocs(c *mgin.Context) {
	index := c.Param("index")

	var docIds []interface{}
	if code, err := c.ReadJSON(&docIds); err != nil {
		c.Error(code, err.Error())
		return
	}

	if err := indexer.DeleteDocs(index, docIds); err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"code": http.StatusOK,
		"msg": "docs removed from index",
		"ids": docIds,
	})
}
