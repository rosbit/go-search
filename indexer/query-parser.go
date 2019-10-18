package indexer

import (
	"strings"
	"fmt"
	"strconv"
)

// 把输入的query参数进行解析，这一步和具体的搜索引擎没有关系
func parseQuery(q, fq, s, f, page, pagesize, fl string) (*parsedQuery, error) {
	var qLabels []string
	qRes, err := parseQ(q)
	if err != nil {
		qLabels = allDocs
	}
	fqRes, err := parseFq(fq)
	if err != nil {
		return nil, err
	}
	fRes, err := parseF(f)
	if err != nil {
		return nil, err
	}

	sRes := parseS(s)
	flRes := parseFl(fl)

	nRows := 20
	if len(pagesize) > 0 {
		nRows, _ = strconv.Atoi(pagesize)
		if nRows <= 0 {
			nRows = 20
		} else if nRows > 100 {
			nRows = 100
		}
	}
	nStart := 0
	if len(page) > 0 {
		if n, err := strconv.Atoi(page); err == nil && n > 0 {
			nStart = (n - 1) * nRows
		}
	}

	return &parsedQuery{
		query: qRes,
		labels: qLabels,
		fquerys: fqRes,
		sortBys: sRes,
		filters: fRes,
		start: nStart,
		rows: nRows,
		outFieldList: flRes,
	}, nil
}

// q: +must should -notIn
func parseQ(q string) (*query, error) {
	fs := fieldsKeepQuote(q)
	if len(fs) == 0 {
		return nil, fmt.Errorf("q parameter expected")
	}

	res := &query{
		should: []string{},
		must:   []string{},
		notIn:  []string{},
	}
	for _, f := range fs {
		switch f[0] {
		case '+':
			if len(f) > 1 {
				res.must = append(res.must, f[1:])
			}
		case '-':
			if len(f) > 1 {
				res.notIn = append(res.notIn, f[1:])
			}
		default:
			res.should = append(res.should, f)
		}
	}

	if len(res.should) == 0 && len(res.must) == 0 && len(res.notIn) == 0 {
		return nil, fmt.Errorf("q parameter expected")
	}

	return res, nil
}

// fq: f1:q-in-field,f2:q-field,...
func parseFq(fq string) ([]fquery, error) {
	fs := fieldsKeepQuote(fq, ',', ';')
	if len(fs) == 0 {
		return nil, nil
	}

	res := []fquery{}
	for _, f := range fs {
		pos := strings.Index(f, ":")
		if pos <= 0 {
			continue
		}
		q, err := parseQ(f[pos+1:])
		if err != nil {
			return nil, err
		}
		if q == nil {
			continue
		}
		res = append(res, fquery{fieldName: f[:pos], query:q})
	}

	if len(res) == 0 {
		return nil, nil
	}
	return res, nil
}

// s: f1:desc,f2:asc
func parseS(s string) []sorting {
	fs := strings.FieldsFunc(s, func(c rune)bool{return (c==',' || c == ';')})

	res := []sorting{}
	for _, f := range fs {
		ss := strings.FieldsFunc(f, func(c rune)bool{return (c==':'||c==' ')})
		if len(ss) == 0 || ss[0] == "" {
			continue
		}

		switch len(ss) {
		case 1: res = append(res, sorting{fieldName: ss[0]})
		default:
			switch ss[1] {
			case "asc":
				res = append(res, sorting{fieldName: ss[0], asc:true})
			default:
				res = append(res, sorting{fieldName: ss[0]})
			}
		}
	}

	if len(res) == 0 {
		return nil
	}
	return res
}

// f: f1:filter1,filter2;f2:filter1,filter2;f3:r1~r2,r3~r4
func parseF(f string) ([]filter, error) {
	fs := fieldsKeepQuote(f, ';')
	if len(fs) == 0 {
		return nil, nil
	}

	res := []filter{}
	for _, f := range fs {
		pos := strings.Index(f, ":")
		if pos < 1 {
			continue
		}

		fRes := filter{fieldName: f[:pos], conds:[]interface{}{}, ranges:[]range_{}}
		conds := fieldsKeepQuote(f[pos+1:], ',')
		for _, cond := range conds {
			pos := strings.Index(cond, "~")
			if pos >= 0 {
				// range
				if len(cond) == 0 {
					continue
				}
				fRes.ranges = append(fRes.ranges, range_{
					from:cond[:pos],
					to:cond[pos+1:],
				})
			} else {
				// filter
				fRes.conds = append(fRes.conds, cond)
			}
		}
		if len(fRes.ranges) == 0 && len(fRes.conds) == 0 {
			continue
		}
		res = append(res, fRes)
	}

	if len(res) == 0 {
		return nil, nil
	}
	return res, nil
}

// fl: f1,f2,...
func parseFl(fl string) []string {
	l := strings.FieldsFunc(fl, func(c rune)bool{return (c==',' || c==' ')})
	if len(l) == 0 {
		return nil
	}
	return l
}
