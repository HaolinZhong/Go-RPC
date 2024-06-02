package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

type GobCodec struct {
	conn io.ReadWriteCloser // connection
	buf  *bufio.Writer      // buffer mitigate blocking
	dec  *gob.Decoder
	enc  *gob.Encoder
}

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buffer := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buffer,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buffer),
	}
}
func (c *GobCodec) ReadHeader(header *Header) error {
	return c.dec.Decode(header)
}

func (c *GobCodec) Close() error {
	return c.conn.Close()
}

func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *GobCodec) Write(header *Header, body interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()

	if err := c.enc.Encode(header); err != nil {
		log.Println("gob error encoding header:", err)
		return err
	}

	if err := c.enc.Encode(body); err != nil {
		log.Println("gob error encoding body:", err)
		return err
	}

	return nil
}
