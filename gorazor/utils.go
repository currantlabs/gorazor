package gorazor

import (
	"fmt"
	"html/template"
	"os"
	"strconv"
	"strings"
	"time"
	"io"
	"bytes"
)

type Section func(w io.Writer)

func EmptySection(io.Writer) {}

func SectionString(s Section) string {
	var _buffer bytes.Buffer
	s(&_buffer)
	return _buffer.String()
}

func HTMLEscape(m interface{}) string {
	s := fmt.Sprint(m)
	return template.HTMLEscapeString(s)
}

func HTMLEscapeWriter(w io.Writer, m interface{}) {
	s := fmt.Sprint(m)
	template.HTMLEscape(w, []byte(s))
}

func StrTime(timestamp int64, format string) string {
	return time.Unix(timestamp, 0).Format(format)
}

func Itoa(obj int) string {
	return strconv.Itoa(obj)
}

func Capitalize(str string) string {
	if len(str) == 0 {
		return ""
	}
	return strings.ToUpper(str[0:1]) + str[1:]
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}
