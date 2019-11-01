package rest

import (
	"github.com/rosbit/http-helper"
	"go-search/indexer"
	"net/http"
	"encoding/json"
	"fmt"
	"io"
	"log"
)

// GET /search/:index?q=+xxx -xxx xxx&s=f1:desc,f2:asc&page=xx&pagesize=xx&f=f1:xxx,r1~r2;f2:r1~r2&fq=f:q-in-field&fl=f1,f2&pretty
//
// 搜索、过滤、排序、输出字段
//
// query arguments:
//  q:  查询条件，+xxx:必出现、-xxx"必不出现、xxx:可以出现
//  fq: 指定字段的q，格式为"字段名:q"，多个fq间用','或';'分割，如fq=name:rosbit;age:10
//  s:  排序字段，格式为"字段名[:desc|asc]"，多个s间用','或';'分割，如s=name;age:asc
//  f:  过滤，支持区间，格式为"字段名:val1,val2,min~max"，min/max可以只出现一个，多个f间用';'分割，如s=name:rosbit,bitros;age:10,16~20,~8,30~
//  page: 页码，从1开始
//  pagesize: 每页条数，最大100
//  fl: 输出字段列表，多个字段名用','分割
//  pretty: 是否美化输出结果，如果没有该参数，则紧凑输出
//
// 返回结果:
// {
//   "code": 200,
//   "msg": "OK",
//   "result":{
//      "timeout": false
//      "docs":[
//          {doc1}, {doc2}, ...
//      },
//      "pagination":{
//        "total": 5,
//        "pages": 1,
//        "page-size": 20,
//        "curr-page": 1,
//        "page-count": 5
//      },
//    }
// }
//
// 注意: ';'必须进行url编码，net/url中';'和'&'的作用是一样的。
func Search(c *helper.Context) {
	log.Printf("[query] %s\n", c.Request().RequestURI)
	index := c.Param("index")

	q  := c.QueryParam("q")
	fq := c.QueryParam("fq")
	s  := c.QueryParam("s")
	f  := c.QueryParam("f")
	page := c.QueryParam("page")
	pagesize  := c.QueryParam("pagesize")
	fl := c.QueryParam("fl")
	_, pretty := c.QueryParams()["pretty"]

	pagination, timeout, docs, err := indexer.Query(index, q, fq, s, f, page, pagesize, fl)
	if err != nil {
		c.Error(http.StatusInternalServerError, err.Error())
		return
	}

	w := c.Response()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if !pretty {
		outputJSONDocByDoc(w, pagination, timeout, docs)
	} else {
		prettyOutputJSONDocByDoc(w, pagination, timeout, docs)
	}
}

func outputJSONDocByDoc(w http.ResponseWriter, pagination interface{}, timeout bool, docs <-chan interface{}) {
	je := json.NewEncoder(w)

	fmt.Fprintf(w, `{"code":%d,"msg":"OK","result":{"timeout":%v,"pagination":`, http.StatusOK, timeout)
	je.Encode(pagination)
	fmt.Fprintf(w, `,"docs":`)
	count := 0
	if docs != nil {
		for doc := range docs {
			if count == 0 {
				fmt.Fprintf(w, "[")
			} else {
				fmt.Fprintf(w, ",")
			}

			je.Encode(doc)
			count += 1
		}
	}

	if count == 0 {
		fmt.Fprintf(w, "null")
	} else {
		fmt.Fprintf(w, "]")
	}
	fmt.Fprintf(w, "}}")
}

func prettyOutputJSONDocByDoc(w http.ResponseWriter, pagination interface{}, timeout bool, docs <-chan interface{}) {
	fmt.Fprintf(w,
`{
  "code": %d,
  "msg": "OK",
  "result": {
    "timeout": %v,
    "pagination": `, http.StatusOK, timeout)

	b, _ := json.MarshalIndent(pagination, "    ", "    ")
	w.Write(b)

	io.WriteString(w, `,
    "docs": `)

	count := 0
	if docs != nil {
		for doc := range docs {
			if count == 0 {
				io.WriteString(w, "[\n      ")
			} else {
				io.WriteString(w, ",\n      ")
			}

			b, _ = json.MarshalIndent(doc, "      ", "    ")
			w.Write(b)
			count += 1
		}
	}

	if count == 0 {
		io.WriteString(w, "null")
	} else {
		fmt.Fprintf(w, "\n    ]")
	}

	fmt.Fprintf(w, `
  }
}
`   )
}
