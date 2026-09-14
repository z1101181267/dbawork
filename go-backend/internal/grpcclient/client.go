// Package grpcclient 封装与 Python 运行时的 gRPC 客户端。
package grpcclient

import (
	"sync"

	pb "dbawork/proto/dbaworkv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// MaxMsgSize 允许的最大消息尺寸（JAR 上传需要 64MB）。
const MaxMsgSize = 64 * 1024 * 1024

// Client 惰性连接 Python gRPC 服务。
type Client struct {
	addr string
	mu   sync.Mutex
	conn *grpc.ClientConn
}

// New 构造客户端。
func New(addr string) *Client { return &Client{addr: addr} }

func (c *Client) ensureConn() (*grpc.ClientConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn, nil
	}
	conn, err := grpc.NewClient(c.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(MaxMsgSize),
			grpc.MaxCallSendMsgSize(MaxMsgSize),
		),
	)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	return conn, nil
}

// Driver 返回驱动管理服务客户端。
func (c *Client) Driver() (pb.DriverServiceClient, error) {
	conn, err := c.ensureConn()
	if err != nil {
		return nil, err
	}
	return pb.NewDriverServiceClient(conn), nil
}

// Connection 返回连接测试服务客户端。
func (c *Client) Connection() (pb.ConnectionServiceClient, error) {
	conn, err := c.ensureConn()
	if err != nil {
		return nil, err
	}
	return pb.NewConnectionServiceClient(conn), nil
}

// Close 关闭底层连接。
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}
