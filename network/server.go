package network

import (
	"Go-RPC/codec"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

const MagicNumber = 0x1a2b3c

type Option struct {
	MagicNumber int
	ContentType codec.ContentType
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	ContentType: codec.GOB,
}

type request struct {
	header       *codec.Header // header of request
	argv, replyv reflect.Value // argv and replyv of request
}

/**

message encoding
| Option{MAGIC_NUMBER: xxx, CodecType: xxx} | Header{ServiceMethod ...} | Body interface{} |
| <------      JSON Encoded      ------>   | <-------   Encoded by ContentType   ------->|

one message can have multiple pairs of header & body
| Option | Header1 | Body1 | Header2 | Body2 | ...

*/

type Server struct{}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer()

func (server *Server) Accept(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("rpc server: accept error:", err)
			return
		}
		go server.ServeConn(conn)
	}
}

func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() {
		_ = conn.Close()
	}()

	// 1. deserialize using Json Decode to get Option
	var option Option
	if err := json.NewDecoder(conn).Decode(&option); err != nil {
		log.Println("rpc server: options error: ", err)
		return
	}

	// 2. check magic number & content type
	if option.MagicNumber != MagicNumber {
		log.Printf("rpc server: invalid magic number %x", option.MagicNumber)
		return
	}

	// 3. based on content type, get corresponding Codec, and pass the decoding task to Codec
	newCodecFunc := codec.CodecConstructorFactory[option.ContentType]
	if newCodecFunc == nil {
		log.Printf("rpc server: invalid codec type %s", option.ContentType)
		return
	}

	server.ServeCodec(newCodecFunc(conn))
}

func (server *Server) ServeCodec(cc codec.Codec) {
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)

	for {
		req, err := server.ReadRequest(cc)
		if err != nil {
			if req == nil {
				break
			}
			req.header.Error = err.Error()
			server.sendResponse(cc, req.header, nil, sending)
			continue
		}
		wg.Add(1)
		go server.handleRequest(cc, req, sending, wg)
	}

	wg.Wait()
	_ = cc.Close()
}

func (server *Server) ReadRequest(cc codec.Codec) (*request, error) {
	header, err := server.ReadRequestHeader(cc)
	if err != nil {
		return nil, err
	}

	req := &request{header: header}

	req.argv = reflect.New(reflect.TypeOf(""))
	if err = cc.ReadBody(req.argv.Interface()); err != nil {
		log.Println("rpc server: read argv err:", err)
	}
	return req, nil
}

func (server *Server) ReadRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var header codec.Header
	if err := cc.ReadHeader(&header); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &header, nil
}

func (server *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println(req.header, req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("rpc response %d", req.header.RequestId))
	server.sendResponse(cc, req.header, req.replyv.Interface(), sending)
}

func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}
