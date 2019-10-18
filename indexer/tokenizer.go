package indexer

import (
	"strings"
	"unicode"
	"fmt"
)

var specialDelis = map[rune]bool{
	'.': true,
	'-': true,
	':': true,
}

func whitespaceTokenize(s string, keepIt ...rune) []string {
	return tokenize_i(s, false, keepIt ...)
}

func hanziTokenize(s string, keepIt ...rune) []string {
	return tokenize_i(s, true, keepIt...)
}

func tokenize_i(s string, breakHz bool, keepIt ...rune) []string {
	if len(s) == 0 {
		return []string{}
	}

	var kp map[rune]bool
	if len(keepIt) > 0 {
		kp = make(map[rune]bool, len(keepIt))
		for _, r := range keepIt {
			kp[r] = true
		}
	}

	tokens := []string{}
	for token := range breakTokens(s, breakHz, kp) {
		tokens = append(tokens, token)
	}

	return tokens
}

func breakTokens(s string, breakHz bool, keepIt map[rune]bool) <-chan string {
	res := make(chan string)
	sb := &strings.Builder{}
	hasQuote := false

	var dumpToken = func() {
		if sb.Len() > 0 {
			var token  string
			// fmt.Printf("keepIt: %v\n", keepIt)
			if len(keepIt) == 0 {
				token = strings.TrimFunc(sb.String(), func(ch rune)bool{
					_, ok := specialDelis[ch]
					return ok
				})
			} else {
				// fmt.Printf("sb.String(): %s\n", sb.String())
				token = strings.TrimFunc(sb.String(), func(ch rune)bool{
					if _, ok := keepIt[ch]; ok {
						return false
					}
					_, ok := specialDelis[ch]
					return ok
				})
			}

			if len(token) > 0 {
				res <- token
			}
			sb.Reset()
		}
	}

	var parseToken = func(s string) {
		var quote rune
		for _, ch := range s {
			switch ch {
			case '\'', '"', '`':
				if hasQuote {
					if quote == ch {
						hasQuote = false
						if len(keepIt) > 0 {
							sb.WriteRune(ch)
						}
						dumpToken()
						break
					}
					sb.WriteRune(ch)
					break
				}

				hasQuote = true
				quote = ch
				dumpToken()
				if len(keepIt) > 0 {
					sb.WriteRune(ch)
				}
			default:
				if hasQuote {
					sb.WriteRune(ch)
					break
				}
				if unicode.IsSpace(ch) {
					dumpToken()
					break
				}
				if unicode.IsPunct(ch) {
					if len(keepIt) == 0 {
						if _, ok := specialDelis[ch]; !ok {
							dumpToken()
							break
						}
					} else {
						if _, ok := keepIt[ch]; !ok {
							if _, ok = specialDelis[ch]; !ok {
								dumpToken()
								break
							}
						}
					}
				}
				if breakHz && unicode.In(ch, unicode.Han) {
					dumpToken()
					res <- fmt.Sprintf("%c", ch)
					break
				}
				sb.WriteRune(ch)
			}
		}
	}

	go func() {
		for {
			parseToken(s)

			if hasQuote {
				hasQuote = false
				s = sb.String()
				sb.Reset()
				if len(keepIt) > 0 {
					s = s[1:]
				}
				continue
			}
			dumpToken()
			break
		}

		close(res)
	}()

	return res
}

func fieldsKeepQuote(s string, deli ...rune) []string {
	hasQuote := false
	var quote rune
	var delis map[rune]bool
	if len(deli) > 0 {
		delis = make(map[rune]bool, len(deli))
		for _, r := range deli {
			delis[r] = true
		}
	}

	return strings.FieldsFunc(s, func(ch rune)bool{
		switch ch {
		case '"', '\'', '`':
			if hasQuote {
				if quote == ch {
					hasQuote = false
				}
				return false
			}

			quote = ch
			hasQuote = true
			return false
		default:
			if hasQuote {
				return false
			}

			if delis == nil {
				return unicode.IsSpace(ch)
			}
			_, ok := delis[ch]
			return ok
		}
	})
}
