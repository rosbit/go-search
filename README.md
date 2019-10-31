# go-search

go-search是基于[github.com/go-ego/riot](https://github.com/go-ego/riot)封装而成的搜索服务。

如果把riot类比java的lucene，go-search相当于solr或elasticsearch。go-search提供了比solr、es
更简单的API接口、更简洁的配置部署方法、占用内存资源更少。

目前go-search还是一个单机版服务，暂时没有做分布式服务的计划。

### 二进制版下载
   Linux下的二进制可执行版本可以直接到[go-search linux](https://github.com/rosbit/go-search/releases)
   下载，另存保存后, `chmod +x go-search`, 就可以使用了

### 编译

   ```bash
$ git clone github.com/rosbit/go-search
$ cd go-search
$ make
   ```

### 运行方法
   1. 需要准备好一个配置文件，参考[sample-conf](sample.conf.json)
        ```json
        {
            "listen-host": "",
            "listen-port": 7080,
            "worker-num": 5,
            "timeout": 0,
            "lru-minutes": 10,          // 至少超过n分钟没访问的索引会从内存清除
            "root-dir": "./schema-home" // 索引配置文件根路径
        }
        ```
        
   1. 环境变量

        - CONF_FILE: 指明配置文件路径
        - USE_STORE: 是否持久化保存索引数据，不指明则索引只保存在内存中，速度快，重启就丢失
             - bg/badger       使用badger持久化，速度较慢
             - ldb/leveldb     使用leveldb持久化，缺省使用，速度快
             - bolt            使用boltdb持久化
        - TZ:  时区，如Asia/Shanghai，缺省时区东8区，对于时间类型的转换保存很重要

1. 运行服务

      ```bash
      $ CONF_FILE=./sample.conf.json USE_STORE=ldb TZ=Asia/Shanghai ./go-search
      ```



### 使用方法

- 参考[API接口文档](go-search.api.md)


### 状态

The package is not fully tested, so be careful.

### Contribution

Pull requests are welcome! Also, if you want to discuss something send a pull request with proposal and changes.
__Convention:__ fork the repository and make changes on your fork in a feature branch.
