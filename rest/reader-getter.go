package rest

import (
	"github.com/rosbit/http-helper"
	"io"
	"fmt"
	"path"
	"strings"
)

func getReader(c *helper.Context, multipartFileParam string) (in io.ReadCloser, contentType, ext string, err error) {
	ct := strings.FieldsFunc(c.Header(HEADER_CONTENT_TYPE), func(ch rune)bool{
		return ch == ' ' || ch == ';'
	})
	if len(ct) > 0 {
		contentType = ct[0]
	}

	switch contentType {
	case MULTIPART_FORM:
		file, e := c.FormFile(multipartFileParam)
		if e != nil {
			err = fmt.Errorf("argument %s expected", multipartFileParam)
		}
		fp, e := file.Open()
		if e != nil {
			err = e
			return
		}
		in = fp
		ext = strings.ToLower(path.Ext(file.Filename))
	default:
		r := c.Request()
		if r.Body == nil {
			err = fmt.Errorf("post body expected")
			return
		}
		in = r.Body
	}
	return
}
