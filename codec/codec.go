package codec

import "io"

type Header struct {
	MethodName string
	RequestId  uint64
	Error      string
}
type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

type NewCodec func(io.ReadWriteCloser) Codec

type ContentType string

const (
	GOB  ContentType = "application/gob"
	JSON ContentType = "application/JSON"
)

var CodecConstructorFactory = make(map[ContentType]NewCodec)

func init() {
	CodecConstructorFactory[GOB] = NewGobCodec
}
