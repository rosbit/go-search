package indexer

import (
	"testing"
	"fmt"
)

var stringsToParse = []string{
	"this is a test",
	"这是,一个 测试 .i.b.m.",
	`这离有"引号 引号里面空格保留",还有不配对引号"  哈哈 还有空格  `,
	`单双引号"引号 引号里面空格保留",还有不配对引号"  哈哈 还有'但引号 带空格'空格  `,
	"更多的测试,2019-10-14 17:02:01",
	"hello, 世界！",
	"日文的之（の）和的（の）有区别吗？",
	`f:"实在"`,
	`f:+"实在"`,
	`f:+"实在" -"有意思"`,
	`f:+"实在" -有意思`,
}

func test_tokenize(tokenize func(string,...rune)[]string) {
	fmt.Printf(" --- token with keeping '+', '-', ':', ',' ... ---\n")
	for _, s := range stringsToParse {
		tokens := tokenize(s, '+', '-', ':', ',')
		fmt.Printf("  + %s => %#v\n", s, tokens)
	}

	fmt.Printf(" --- token it .. ---\n")
	for _, s := range stringsToParse {
		tokens := tokenize(s)
		fmt.Printf("  + %s => %#v\n", s, tokens)
	}
}

func Test_hzTokenizer(t *testing.T) {
	fmt.Printf("=== begin hanziTokenize testing...\n")
	test_tokenize(hanziTokenize)
}

func Test_wsTokenizer(t *testing.T) {
	fmt.Printf("=== begin whitespaceTokenize testing...\n")
	test_tokenize(whitespaceTokenize)
}

func Test_fieldsWithQuote(t *testing.T) {
	fmt.Printf("=== begin fieldsWithQuote ...\n")
	test_tokenize(fieldsKeepQuote)
}
