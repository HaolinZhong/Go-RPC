package network

import (
	"Go-RPC/main/codec"
	"Go-RPC/main/service"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"
)

const MagicNumber = 0x1a2b3c

type Option struct {
	MagicNumber    int
	ContentType    codec.ContentType
	ConnectTimeout time.Duration
	HandleTimeout  time.Duration
}

var DefaultOption = &Option{
	MagicNumber:    MagicNumber,
	ContentType:    codec.GOB,
	ConnectTimeout: time.Second * 10,
}

type request struct {
	header       *codec.Header // header of request
	argv, replyv reflect.Value // argv and replyv of request
	mType        *service.MethodType
	svc          *service.Service
}

/**

message encoding
| Option{MAGIC_NUMBER: xxx, CodecType: xxx} | Header{ServiceMethod ...} | Body interface{} |
| <------      JSON Encoded      ------>   | <-------   Encoded by ContentType   ------->|

one message can have multiple pairs of header & body
| Option | Header1 | Body1 | Header2 | Body2 | ...

*/

type Server struct {
	serviceMap sync.Map
}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer()

// invalidRequest is a placeholder for response argv when error occurs
var invalidRequest = struct{}{}

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
	newCodecFunc := codec.CodecConstructorMap[option.ContentType]
	if newCodecFunc == nil {
		log.Printf("rpc server: invalid codec type %s", option.ContentType)
		return
	}

	server.ServeCodec(newCodecFunc(conn), &option)
}

func (server *Server) ServeCodec(cc codec.Codec, opt *Option) {
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
		go server.handleRequest(cc, req, sending, wg, opt.HandleTimeout)
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
	req.svc, req.mType, err = server.FindService(header.ServiceMethodName)

	if err != nil {
		return req, err
	}

	req.argv = req.mType.NewArgV()
	req.replyv = req.mType.NewReplyV()

	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}
	if err = cc.ReadBody(argvi); err != nil {
		log.Println("rpc server: read body err:", err)
		return req, err
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

func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	called := make(chan struct{})
	sent := make(chan struct{})

	go func() {
		err := req.svc.Call(req.mType, req.argv, req.replyv)
		called <- struct{}{}
		if err != nil {
			req.header.Error = err.Error()
			server.sendResponse(cc, req.header, invalidRequest, sending)
			return
		}
		server.sendResponse(cc, req.header, req.replyv.Interface(), sending)
		sent <- struct{}{}
	}()

	if timeout == 0 {
		<-called
		<-sent
		return
	}

	select {
	case <-time.After(timeout):
		req.header.Error = fmt.Sprintf("rpc server: request handle timeout: expect within %s", timeout)
		server.sendResponse(cc, req.header, invalidRequest, sending)
	case <-called:
		<-sent
	}
}

func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}

func (server *Server) Register(rcvr interface{}) error {
	s := service.NewService(rcvr)
	if _, dup := server.serviceMap.LoadOrStore(s.Name, s); dup {
		return errors.New("rpc: service already defined: " + s.Name)
	}
	return nil
}

func Register(rcvr interface{}) error {
	return DefaultServer.Register(rcvr)
}

func (server *Server) FindService(serviceMethodName string) (svc *service.Service, mType *service.MethodType, err error) {
	dot := strings.LastIndex(serviceMethodName, ".")
	if dot < 0 {
		err = errors.New("rpc server: service/method request ill-formed: " + serviceMethodName)
		return
	}

	serviceName, methodName := serviceMethodName[:dot], serviceMethodName[dot+1:]

	svci, ok := server.serviceMap.Load(serviceName)

	if !ok {
		err = errors.New("rpc server: can't find service " + serviceName)
		return
	}

	svc = svci.(*service.Service)
	mType = svc.Method[methodName]

	if mType == nil {
		err = errors.New("rpc server: can't find method " + methodName)
	}

	return
}
