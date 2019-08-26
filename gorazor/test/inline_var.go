package cases

import (
	"bytes"
	"github.com/currantlabs/gorazor/gorazor"
)

type Author struct {
	Name string
	Age int
}

func Inline_var() string {
	var _buffer bytes.Buffer
	_buffer.WriteString("\n\n<body>")
	_buffer.WriteString(gorazor.HTMLEscape(Hello("Felix Sun", "h1", 30, &Author{"Van", 20}, 10)))
	_buffer.WriteString("\n</body>")

	return _buffer.String()
}
