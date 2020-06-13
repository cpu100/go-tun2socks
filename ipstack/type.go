package ipstack

import (
    "io"
    "net"
)

type Cptr *[0]byte

type TunHandler interface {
    io.ReadWriteCloser
    TcpHandle(conn net.Conn) error
    UdpHandle(conn net.Conn) error
}
