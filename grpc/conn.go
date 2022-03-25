package grpc

import (
	"bytes"
	"context"
	"io"
	"net"
	"time"

	"github.com/hahahrfool/v2ray_simple/utils"
)

// StreamConn 接口 是 stream_grpc.pb.go 中 自动生成的 Stream_TunServer 接口和 Stream_TunClient接口 的共有部分, 我们提出来.
type StreamConn interface {
	Context() context.Context
	Send(*Hunk) error
	Recv() (*Hunk, error)
}

// Conn implements net.Conn
type Conn struct {
	stream      StreamConn
	cacheReader io.Reader
	closeFunc   context.CancelFunc
	local       net.Addr
	remote      net.Addr
}

func (c *Conn) Read(b []byte) (n int, err error) {
	//这里用到了一种缓存方式
	if c.cacheReader == nil {
		h, err := c.stream.Recv()
		if err != nil {
			return 0, utils.NewErr("unable to read from gun tunnel", err)
		}
		c.cacheReader = bytes.NewReader(h.Data)
	}
	n, err = c.cacheReader.Read(b)
	if err == io.EOF {
		c.cacheReader = nil
		return n, nil
	}
	return n, err
}

func (c *Conn) Write(b []byte) (n int, err error) {
	err = c.stream.Send(&Hunk{Data: b})
	if err != nil {
		return 0, utils.NewErr("Unable to send data over stream service", err)
	}
	return len(b), nil
}

func (c *Conn) Close() error {
	if c.closeFunc != nil {
		c.closeFunc()
	}
	return nil
}
func (c *Conn) LocalAddr() net.Addr {
	return c.local
}
func (c *Conn) RemoteAddr() net.Addr {
	return c.remote
}
func (*Conn) SetDeadline(time.Time) error {
	return nil
}
func (*Conn) SetReadDeadline(time.Time) error {
	return nil
}
func (*Conn) SetWriteDeadline(time.Time) error {
	return nil
}

// NewConn creates Conn which handles StreamConn.
// 需要一个 cancelFunc 参数, 是因为 在 处理下一层连接的时候(如vless), 有可能出问题(如uuid不对), 并需要关闭整个 grpc连接. 我们只能通过 chan 的方式（即cancelFunc）来通知 上层进行关闭.
func NewConn(service StreamConn, cancelFunc context.CancelFunc) *Conn {
	conn := &Conn{
		stream:      service,
		cacheReader: nil,
		closeFunc:   cancelFunc,
	}

	conn.local = &net.TCPAddr{
		IP:   []byte{0, 0, 0, 0},
		Port: 0,
	}
	ad := addrFromContext(service.Context())
	if ad != nil {
		conn.remote = ad
	} else {
		conn.remote = &net.TCPAddr{
			IP:   []byte{0, 0, 0, 0},
			Port: 0,
		}
	}

	return conn
}

//只是用于 addrFromContext 而已
type AuthInfo interface {
	AuthType() string
}

// peer contains the information of the peer for an RPC.
type peer struct {
	// Addr is the peer address.
	Addr net.Addr
	// AuthInfo is the authentication information of the transport.
	// It is nil if there is no transport security being used.
	AuthInfo AuthInfo
}

type peerKey struct{}

func addrFromContext(ctx context.Context) net.Addr {
	p, ok := ctx.Value(peerKey{}).(*peer)

	if !ok {
		return nil
	}
	return p.Addr
}
