// global conf
//  ENV:
//   CONF_FILE        --- 配置文件名
//   USE_STORE        --- 是否使用持久化
//   TZ               --- 时区名称"Asia/Shanghai"
//
// $CONF_FILE in JSON:
// {
//	"listen-host": "",
//	"listen-port": 7080,
//	"worker-num": 5,
//	"timeout": 0,
//	"root-dir": "/path/to/root",
//	"lru-minutes": 1,
//	"seg-dict" {
//		"dict-file": "/path/to/dict-file",
//		"stop-file": "/path/to/stopword-file"
//	}
// }
//
// Rosbit Xu
package conf

import (
	"fmt"
	"os"
	"time"
	"encoding/json"
)

var (
	// 全局配置信息
	ServiceConf struct {
		ListenHost string `json:"listen-host"`
		ListenPort int    `json:"listen-port"`
		WorkerNum  int    `json:"worker-num"`
		Timeout    int    `json:"timeout"`
		RootDir    string `json:"root-dir"`
		LruMinutes int    `json:"lru-minutes"`
		SegDict struct {
			DictFile   string `json:"dict-file"`
			StopFile   string `json:"stop-file"`
		} `json:"seg-dict"`
	}

	// 缺省时区，会被环境变量TZ覆盖
	Loc = time.FixedZone("UTC+8", 8*60*60)

	UseStore string

	validStore = map[string]string{
		"bg": "bg",
		"badger": "bg",
		"ldb": "ldb",
		"leveldb": "ldb",
		"bolt": "bolt",
	}
	defaultStore = "ldb"
)

func getEnv(name string, result *string, must bool) error {
	s := os.Getenv(name)
	if s == "" {
		if must {
			return fmt.Errorf("env \"%s\" not set", name)
		}
	}
	*result = s
	return nil
}

// 检查全局配置信息
func CheckGlobalConf() error {
	var p string
	getEnv("TZ", &p, false)
	if p != "" {
		if loc, err := time.LoadLocation(p); err == nil {
			Loc = loc
		}
	}

	getEnv("USE_STORE", &p, false)
	if p != "" {
		if db, ok := validStore[p]; ok {
			UseStore = db
		} else {
			UseStore = defaultStore
		}
	}

	var confFile string
	if err := getEnv("CONF_FILE", &confFile, true); err != nil {
		return err
	}

	fp, err := os.Open(confFile)
	if err != nil {
		return err
	}
	defer fp.Close()
	dec := json.NewDecoder(fp)
	if err = dec.Decode(&ServiceConf); err != nil {
		return err
	}

	if err = checkMust(); err != nil {
		return err
	}

	if ServiceConf.LruMinutes <= 0 {
		ServiceConf.LruMinutes = 10
	}

	return nil
}

func checkMust() error {
	if ServiceConf.ListenPort <= 0 {
		return fmt.Errorf("listening port expected in conf")
	}

	if ServiceConf.RootDir == "" {
		return fmt.Errorf("root-dir expected in conf")
	}
	fi, err := os.Stat(ServiceConf.RootDir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", ServiceConf.RootDir)
	}

	/*
	segDict := &ServiceConf.SegDict
	if err := checkDict(segDict.DictFile, "seg-dict/dict-file"); err != nil {
		return err
	}
	if err := checkDict(segDict.StopFile, "seg-dict/stop-file"); err != nil {
		return err
	}*/

	return nil
}

func checkDict(path, prompt string) error {
	if path == "" {
		return fmt.Errorf("%s expected in conf", prompt)
	}

	_, err := os.Stat(path)
	return err
}

// 显示全局配置信息
func DumpConf() {
	fmt.Printf("conf: %v\n", ServiceConf)
	fmt.Printf("TZ time location: %v\n", Loc)
	fmt.Printf("UseStore: %v\n", UseStore)
}

