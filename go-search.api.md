# go-search HTTP API说明文档



## 一、schema维护

### 1.1 创建schema

- URI: /schema/:index

- 方法: POST

- 路径参数名

  - :index  索引库名称

- 请求头

  - Content-Type: multipart/form-data

    或者

  - Content-Type: application/json

- 请求体

  - 如果是multipart上传文件，上传文件的参数名为”file"，内容为一个JSON，JSON格式见下面

  - 如果是非multipart请求，请求体的内容为JSON，例子及说明：

    ```json
    {
      "shard": 8,   // 这个参数指定索引分片数目，如果没有该参数，数目为8
      "fields": [
        {
          "name": "id",     // 字段名称，必须给定
          "type": "u32",    // 数据类型，如果不指定缺省为"str"
                            // 这是做索引库中保存的数据类型，索引文档中的类型不需要完全匹配该类型
          "pk": true        // 是否为主键，可以有多个字段标明是"pk"
        },
        {
          "name": "name",
          "tokenizer": "zh" // 字符串分词方法，可以有"zh","space"或"none"，缺省为"space"
        },
        {
          "name": "age",
          "type": "u16"
        },
        {
          "name": "tags",
          "tokenizer": "space"
        },
        {
          "name": "update-time",
          "type": "datetime",// "date","time","datetime"可以通过属性"time-fmt"指明格式
                             // 缺省格式分别为"2006-01-02","15:04:05","2006-01-02 15:04:05"
          "sorting": "desc"  // 缺省排序字段，如果没有一个sorting字段，结果按主键升序排列
        }
      ]
    }
    ```

  - 可以使用的数据类型

    | 类型                   | 说明                                                         | 例子                                                         |
    | ---------------------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
    | str,string             | UTF-8字符串                                                  | "世界", "world"                                              |
    | i8, i16, i32, i64, int | 8,16,32,64位有符号整型值                                     | 10, -200                                                     |
    | u8,u16,u32,u64,uint    | 8,16,32,64位无符号整型值                                     | 128, 65535                                                   |
    | f32,f64,float          | 单/双精度浮点数                                              | 1.0, 3.1415                                                  |
    | bool,boolean           | 布尔值                                                       | true,false<br />添加索引时doc中的布尔值可以用字符串和整数表示<br />""(空串)、null 会转为false<br />"y","yes","true" 会转为true<br />0表示false，非0整数表示为true |
    | time                   | 时间类型，缺省时间格式为"15:04:05"，可以通过属性"time-fmt"指明 | "09:01:58"                                                   |
    | date                   | 日期类型，缺省时间格式为"2006-01-02"，可以通过属性"time-fmt"指明 | "2019-10-17"                                                 |
    | datetime               | 日期时间类型，缺省时间格式"2006-01-02 15:04:05"，可以通过属性"time-fmt"指明 | "2019-10-17 14:42:59"                                        |
    | json                   | 可以任何的内嵌JSON                                           | null, 10, {"a":1, "b": "c"}                                  |



### 1.2 删除schema

- URI: /schema/:index
- 方法: DELETE
- 路径参数
  - :index 要删除的索引库名
- 功能：该接口会删除索引库schema以及所有已经索引的数据



### 1.3 查询schema

- URI: /schema/:index
- 方法: GET
- 路径参数
  - :index 要查询的索引库名
- 功能: 如果:index存在，显示schema配置信息



## 二、索引增删改

说明：

- go-search中的索引是以**文档**(doc)为单位的

- 一个**文档**可以类比成数据库表的一条记录，一个**文档**可以有多个**字段**

- 每个doc对应一个__docid__，由go-search生成，该id是由所有的__主键__拼接而成的，是一个字符串

- **docid**的拼接规则：把每个主键转换成字符串，然后用"_"连接

- 只有出现在索引库的schema中的字段才会进入索引库

- 如果需要更新go-search索引库中的一个文档，再次调用__增加__接口就可以了

  

### 2.1 增加单个索引文档

- URI: /doc/:index

- 方法: PUT

- 路径参数

  - :index 要更新的索引库名

- 请求头

  - Content-Type: application/json

- 请求体

  - 文档内容，是一个JSON，例：

  ```json
  {
      "id": 1,
      "name": "this ia a test",
      "age": 10,
      "tags": "just test",
      "update-time": "2019-10-10 19:01:48"
  }
  ```

- 成功的返回格式

  ```json
  {
     "code": 200,
     "msg": "doc added to index",
     "id": "value-of-docid" // 这个值是文档的docid
  }
  ```

  

### 2.2 批量增加索引文档

- URI: /docs/:index[?cb=url-to-callback]

- 方法 PUT

- 路径参数

  - :index 要更新的索引库名

- query参数:

  - cb 可选参数，是一个url编码的回调接口地址

- 请求头和请求体

  - 请求头Content-Type指明body类型，可取值和对应的body

    | Content-Type值       | body内容                                                     |
    | :------------------- | :----------------------------------------------------------- |
    | multipart/form-data  | 上传文件内容，参数名“file”，上传文件扩展名必须是“.json”、".csv"或".jsonl"，文件内容参考下面的说明 |
    | application/json     | JSON数组，每一项是一个文档，参考“增加单个索引文档”。未指明Conent-Type按JSON数组处理 |
    | text/csv             | csv文件，第一行是字段名，其余每行是一个文档                  |
    | application/x-ndjson | JSON Lines文件，一般每行(可以占多行)一个JSON，JSON间不需要用','分隔，每个JSON是一个文档 |

- 返回

  - 如果请求不带cb参数，成功的返回结果为

    ```json
    {
        "code": 200,
        "msg": "docs added to index",
        "ids": [
            "doid",          // 对于成功加进索引库的文档，这个值是docid
            "error-message", // 对于加索引失败的文档，这个值是错误信息
            "other-docid",
            "other-error-message"
        ]
    }
    ```

  - 如果带cb参数，则返回结果为
  
    ```json
    {
        "code": 200,
        "msg": "indexing request accepted"
    }
    ```
  
  - 在索引完成后，会以POST方式请求cb参数
  
    - 请求头
  
      - Content-Type: application/json
  
    - 请求体格式
  
      - 成功返回
  
        ```json
        {
            "code": 200,
            "msg": "OK",
            "index": ":index参数，即索引库名"
        }
        ```
  
      - 发生错误时返回
  
        ```json
        {
            "code": 500,
            "msg": "failed to index docs",
            "index": ":index参数，即索引库名"
        }
        ```
  
        

### 2.3 删除单个索引文档

- URI: /doc/:index

- 方法: DELETE

- 路径参数

  - :index 索引库名

- 请求头

  - Content-Type: application/json

- 请求体

  ```json
  {
     "id": "value-of-docid-to-be-removed"
  }
  ```

  

### 2.4 批量删除索引文档

- URL: /docs/:index

- 方法: DELETE

- 路径参数

  - :index 索引库名

- 请求头

  - Content-Type: application/json

- 请求体

  ```json
  [
     "docid1", "docId2", "..."
  ]
  ```

  

## 三、查询接口及语法

- URI: /search/:index?q=query&s=sorting&page=page-no&pagesize=page-size&f=filter&fq=field-query&fl=field-list

- 方法：GET

- 参数说明

  | 参数     | 说明                                                         | 例子                                                         |
  | -------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
  | q        | 查询串，多个串用空格分隔<br />+xxx: xxx必出现，-xxx: xxx必不出现<br />查询串可以加引号防止被分词 | 1. q=+rosbit<br />2. q=“世界”                                |
  | s        | 字段排序条件，多个排序条件用','分隔<br />基本格式: "字段名:asc\|desc"<br />如果只有字段名，排序方式为desc | s=age:asc,update-time<br />表示先按“age"升序，再按"udpate-time"降序 |
  | f        | 按字段过滤，基本格式: "字段名:过滤条件"<br />同一字段内多个条件为“或”关系，用','分隔<br />多个字段过滤条件为"与"关系，用';'分隔<br />过滤条件可以是区间范围，区间的两个边界值用'~'分隔，可以只出现一个边界值 | f=age:10,12~15,20~;tags:"学生"<br />表示tags包含“学生”、年龄为10, 12<=x<=15, 20及以上 |
  | fq       | 在字段内查询，是参数q的更一般形式，基本格式为："字段名:查询串"，多个查询串用','分隔 | fq=tags:世界                                                 |
  | fl       | 需要输出的字段名，用','分隔。如果没有该参数输出doc的全部字段 | fl=id,age,name                                               |
  | page     | 页码，从1开始计数，缺省为1                                   | page=10                                                      |
  | pagesize | 每页结果数，最大100，缺省为20                                | pagesize=5                                                   |

- 返回结果

  ```json
  {
    "code": 200,
    "msg": "OK",
    "result":{
       "timeout": false,
       "pagination":{
          "total": 1,      // 满足条件的结果数
          "pages": 1,      // 总页数
          "page-size": 20, // 每页条数
          "curr-page": 1,  // 返回结果的当前页码
          "page-count": 1  // 当前页中的结果数
       },
       "docs":[
         {"age": 20, "id": 3, "name": "this is a test", "tags": "测试 test",…}
       ]
     }
  }
  ```

  