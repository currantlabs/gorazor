package cases

import (
	"bytes"
	"github.com/currantlabs/gorazor/gorazor"
)

func Email() string {
	var _buffer bytes.Buffer
	_buffer.WriteString("<span>rememberingsteve@apple.com ")
	_buffer.WriteString(gorazor.HTMLEscape(username))
	_buffer.WriteString("</span>")

	return _buffer.String()
}
