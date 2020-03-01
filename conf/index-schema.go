// schema文件解析
// 格式:
// {
//    "name": "hello",
//    "shards": 8,
//    "fields": [
//        {
//            "name": "f1",
//            "pk": true|false, // 属于PK的字段一定会保存
//            "type": "string"|"i8"|"u8"|...|"float"|"date"|"datetime"|"time"|"timestamp", // timestamp单位秒，是i64的别名
//            "tokenizer": "zh"|"space"|"none"|null, // 分词器：中文、空白、不需要；只有字符串有效
//            "time-fmt": "",    // 当type是date,datetime,time时的格式串，缺省分别为"YYYY-MM-DD", "YYYY-MM-DD HH:MM:SS", "HH:MM:SS"，可以精确到毫秒
//            "sorting": "desc"|"asc"  // 参与没有排序条件时的缺省排序
//        },
//        {
//            "name":"f2",
//            ....
//        }
//     ]
//}
package conf

import (
	"path"
	"fmt"
	"os"
	"io"
	"time"
	"encoding/json"
	"strconv"
	"strings"
	"reflect"
)

// 各种分词器
const (
	ZH_TOKENIZER   = "zh"
	WS_TOKENIZER   = "space" // default tokenizer
	NONE_TOKENIZER = "none"
)

var (
	// 所有合法的类型名
	validTypes = map[string]bool{
		"str":true,"string":true,
		"i8":true, "i16":true, "i32":true, "i64":true, "int":true, "integer":true,
		"u8":true, "u16":true, "u32":true, "u64":true, "uint":true,
		"f32": true, "f64":true, "float":true,
		"bool":true, "boolean":true,
		"date":true, "datetime":true, "time":true,"timestamp":true,
		"json":true,
	}

	defaultTimeLayouts = map[string]string {
		"date": "2006-01-02",
		"time": "15:04:05",
		"datetime": "2006-01-02 15:04:05",
	}
)

// 字段定义
type Field struct {
	Name      string `json:"name"`
	PK        bool   `json:"pk"`
	Type      string `json:"type"`
	TimeFmt   string `json:"time-fmt,omitempty"`
	Tokenizer string `json:"tokenizer"`
	Sorting   string `json:"sorting,omitempty"`
}

// schema字段列表
type SchemaConf struct {
	Shards  uint16  `json:"shards"`
	Fields  []Field `json:"fields"`
}

// 缺省排序列表
type DefSorting struct {
	FieldIdx  int
	Ascending bool
}

// schema定义，一个索引库一个schema
type Schema struct {
	// 索引库名称
	Name      string

	// 保存索引库的路径
	StorePath string

	// schema配置
	*SchemaConf

	// 字段名 -> 字段序号
	FieldMap  map[string]int

	// PK字段的序号，PK可以是组合
	PKIdx []int

	// 缺省排序
	DefSortBys []DefSorting

	// 需要转换输出的时间字段
	TimeIdx map[string]int

	// 是否需要中文分词
	NeedZhSeg bool
}

// 加载一个索引库的schema
//   index: 索引库名
func LoadSchema(index string) (*Schema, error) {
	d, p := generateSchemaFile(index)
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	schemaConf, err := parseSchema(f)
	if err != nil {
		return nil, err
	}

	fm, pi, defSortBys, ti, needZhSeg, err := checkSchemaConf(index, schemaConf)
	if err != nil {
		return nil, err
	}
	return &Schema{
		Name:       index,
		StorePath:  d,
		SchemaConf: schemaConf,
		FieldMap:   fm,
		PKIdx:      pi,
		DefSortBys: defSortBys,
		TimeIdx:    ti,
		NeedZhSeg:  needZhSeg,
	}, nil
}

// 保存一个索引库的schema
//   index: 索引库名
func SaveSchema(index string, in io.Reader) error {
	schemaConf, err := parseSchema(in)
	if err != nil {
		return err
	}
	if _, _, _, _, _, err = checkSchemaConf(index, schemaConf); err != nil {
		return err
	}

	d, p := generateSchemaFile(index)
	if err = createDir(d); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("  ", "  ")
	return enc.Encode(schemaConf)
}

// 删除一个索引库的schema
//   index: 索引库名
// 该函数会删除schema文件及所有已经生成的索引文件
func DeleteSchema(index string) error {
	d, _ := generateSchemaFile(index)
	if _, err := os.Stat(d); err != nil && os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(d)
}

// 索引库改名
//   index: 原索引名
//   newIndex: 新索引名
func RenameSchema(index, newIndex string) error {
	d, _ := generateSchemaFile(index)
	nd, _ := generateSchemaFile(newIndex)
	return os.Rename(d, nd)
}

// 把日期、时间字段格式化输出
func (field *Field) FormatDatetime(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	nsec, ok := v.(int64)
	if !ok {
		return nil
	}

	switch field.Type {
	case "date", "datetime", "time":
		return time.Unix(0, nsec).In(Loc).Format(field.TimeFmt)
	default:
		return nil
	}
}

// 根据字段类型把给定的字段值转换为相应的类型
//    value:   需要转换的值
// 返回的数据中已经是经过转换的数据
func (field *Field) ToNativeValue(value interface{}) (interface{}, error) {
	switch field.Type {
	case "str", "string":
		if value == nil {
			return "", nil
		}
		return fmt.Sprintf("%v", value), nil
	case "i8":
		if i, err := toInt(value); err != nil {
			return nil, err
		} else {
			return int8(i), nil
		}
	case "i16":
		if i, err := toInt(value); err != nil {
			return nil, err
		} else {
			return int16(i), nil
		}
	case "i32":
		if i, err := toInt(value); err != nil {
			return nil, err
		} else {
			return int32(i), nil
		}
	case "i64", "timestamp":
		return toInt(value)
	case "int", "integer":
		if i, err := toInt(value); err != nil {
			return nil, err
		} else {
			return int(i), nil
		}
	case "u8":
		if i, err := toUint(value); err != nil {
			return nil, err
		} else {
			return uint8(i), nil
		}
	case "u16":
		if i, err := toUint(value); err != nil {
			return nil, err
		} else {
			return uint16(i), nil
		}
	case "u32":
		if i, err := toUint(value); err != nil {
			return nil, err
		} else {
			return uint32(i), nil
		}
	case "u64":
		return toUint(value)
	case "uint":
		if i, err := toUint(value); err != nil {
			return nil, err
		} else {
			return uint(i), nil
		}
	case "f32":
		if i, err := toFloat(value); err != nil {
			return nil, err
		} else {
			return float32(i), nil
		}
	case "f64", "float":
		if i, err := toFloat(value); err != nil {
			return nil, err
		} else {
			return float64(i), nil
		}
	case "date", "datetime", "time":
		return toDatetime(value, field.TimeFmt)
	case "bool", "boolean":
		return toBool(value)
	case "json":
		return value, nil
	default:
		return nil, fmt.Errorf("unknown data type %s", field.Type)
	}
}

func toInt(v interface{}) (int64, error) {
	if v == nil {
		return 0, nil
	}
	switch v.(type) {
	case float64:
		return int64(v.(float64)), nil
	case string:
		s := v.(string)
		if s == "" {
			return 0, nil
		}
		return strconv.ParseInt(s, 10, 64)
	case int8, int16, int32, int64, int:
		return reflect.ValueOf(v).Int(), nil
	default:
		return 0, fmt.Errorf("can not convert %v to int64", v)
	}
}

func toUint(v interface{}) (uint64, error) {
	if v == nil {
		return 0, nil
	}
	switch v.(type) {
	case float64:
		return uint64(v.(float64)), nil
	case string:
		s := v.(string)
		if s == "" {
			return 0, nil
		}
		return strconv.ParseUint(s, 10, 64)
	case uint8, uint16, uint32, uint64, uint:
		return reflect.ValueOf(v).Uint(), nil
	default:
		return 0, fmt.Errorf("can not convert %v to uint64", v)
	}
}

func toFloat(v interface{}) (float64, error) {
	if v == nil {
		return float64(0), nil
	}
	switch v.(type) {
	case float64:
		return v.(float64), nil
	case float32:
		return float64(v.(float32)), nil
	case string:
		s := v.(string)
		if s == "" {
			return float64(0), nil
		}
		return strconv.ParseFloat(s, 64)
	default:
		return 0.0, fmt.Errorf("can not convert %v to float64", v)
	}
}

var trueVals = map[string]bool{"yes":true, "y":true, "true": true}
func toBool(v interface{}) (bool, error) {
	if v == nil {
		return false, nil
	}
	switch v.(type) {
	case bool:
		return v.(bool), nil
	case float64:
		return int(v.(float64)) != 0, nil
	case string:
		s := v.(string)
		if s == "" {
			return false, nil
		}
		if i, err := strconv.Atoi(s); err == nil {
			return i != 0, nil
		} else {
			_, ok := trueVals[strings.ToLower(s)];
			return ok, nil
		}
	default:
		return false, fmt.Errorf("can not convert %v to boolean", v)
	}
}

func toDatetime(v interface{}, timeFmt string) (int64, error) {
	if v == nil {
		return 0, nil
	}
	switch v.(type) {
	case float64:
		return int64(v.(float64)), nil
	case string:
		if t, err := time.ParseInLocation(timeFmt, v.(string), Loc); err != nil {
			return 0, err
		} else {
			return t.UnixNano(), nil
		}
	default:
		return 0, fmt.Errorf("connot convert %v to time", v)
	}
}

func parseSchema(in io.Reader) (*SchemaConf, error) {
	dec := json.NewDecoder(in)
	var schemaConf SchemaConf
	if err := dec.Decode(&schemaConf); err != nil {
		return nil, err
	}
	return &schemaConf, nil
}

func createDir(d string) error {
	fi, err := os.Stat(d)
	if err != nil && os.IsNotExist(err) {
		return os.Mkdir(d, 0755)
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory")
	}
	return nil
}

func generateSchemaFile(index string) (d, f string) {
	d = path.Join(ServiceConf.RootDir, index)
	f = fmt.Sprintf("%s/schema.json", d)
	return
}

func checkSchemaConf(name string, schemaConf *SchemaConf) (map[string]int, []int, []DefSorting, map[string]int, bool, error) {
	if schemaConf.Fields == nil || len(schemaConf.Fields) == 0 {
		return nil, nil, nil, nil, false, fmt.Errorf("no fields found in %s schema file", name)
	}

	needZhSeg := false
	l := len(schemaConf.Fields)
	fm := make(map[string]int, l)
	pi := []int{}
	var defSorting []DefSorting
	ti := make(map[string]int, l)
	for i:=0; i<l; i++ {
		field := &schemaConf.Fields[i]
		if field.Name == "" {
			return nil, nil, nil, nil, false, fmt.Errorf("no name for field #%d in %s schema file", i, name)
		}
		if fn, ok := fm[field.Name]; ok {
			return nil, nil, nil, nil, false, fmt.Errorf("field name %s duplicated, field #%d,#%d are same", field.Name, fn, i)
		}

		switch field.Type {
		case "": field.Type = "str"
		case "date", "time", "datetime":
			ti[field.Name] = i
			if field.TimeFmt == "" {
				field.TimeFmt, _ = defaultTimeLayouts[field.Type]
			}
		default:
			if _, ok := validTypes[field.Type]; !ok {
				return nil, nil, nil, nil, false, fmt.Errorf("invalid type name %s for field %s", field.Type, field.Name)
			}
		}

		switch field.Tokenizer {
		case ZH_TOKENIZER: needZhSeg = true
		case WS_TOKENIZER:
		case NONE_TOKENIZER:
		case "": field.Tokenizer = WS_TOKENIZER
		default:
			return nil, nil, nil, nil, false, fmt.Errorf("unknown tokenizer %s in field name %s", field.Tokenizer, field.Name)
		}

		if field.PK {
			pi = append(pi, i)
		}

		switch field.Sorting {
		case "desc":
			defSorting = append(defSorting, DefSorting{
				FieldIdx: i,
				Ascending: false,
			})
		case "asc":
			defSorting = append(defSorting, DefSorting{
				FieldIdx: i,
				Ascending: true,
			})
		default:
		}

		fm[field.Name] = i
	}

	if len(pi) == 0 {
		return nil, nil, nil, nil, false, fmt.Errorf("no PK field(s) specified")
	}

	if schemaConf.Shards == 0 {
		schemaConf.Shards = 8
	}
	if len(ti) == 0 {
		ti = nil
	}
	return fm, pi, defSorting, ti, needZhSeg, nil
}
