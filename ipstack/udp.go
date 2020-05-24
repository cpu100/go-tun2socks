package ipstack

/*
#include "c/include/lwip/udp.h"
#include "c/custom/tool/tool.h"
*/
import "C"
import (
    "errors"
    "fmt"
    "io"
    "net"
    "time"
    "unsafe"
)

type UDPConn struct {
    localAddr  *net.UDPAddr
    remoteAddr *net.UDPAddr

    chRead chan []byte

    readDeadline  time.Time
    writeDeadline time.Time

    is  *ipStack
    pcb *C.struct_udp_pcb
}

// func (conn *UDPConn) WriteTo(w io.Writer) (n int64, err error) {
//     panic("implement me")
// }
//
// func (conn *UDPConn) ReadFrom(r io.Reader) (n int64, err error) {
//     panic("implement me")
// }

func (conn *UDPConn) Read(b []byte) (n int, err error) {
    bs := <-conn.chRead
    if nil == bs {
        return 0, io.EOF
    }
    if len(b) < len(bs) {
        return 0, io.ErrShortBuffer
    }
    copy(b, bs)
    return len(bs), nil
}

func (conn *UDPConn) Write(b []byte) (int, error) {
    buf := C.pbuf_alloc_reference(unsafe.Pointer(&b[0]), C.u16_t(len(b)), C.PBUF_ROM)
    defer C.pbuf_free(buf)

    conn.is.Lock()
    defer conn.is.Unlock()

    if len(conn.localAddr.IP) == 4 {
        C.ip_addr_type_set(&conn.pcb.local_ip, C.IPADDR_TYPE_V4)
        C.ip_addr_type_set(&conn.pcb.remote_ip, C.IPADDR_TYPE_V4)
        copy(conn.pcb.local_ip.u_addr[:4], conn.remoteAddr.IP)
        copy(conn.pcb.remote_ip.u_addr[:4], conn.localAddr.IP)
    } else {
        C.ip_addr_type_set(&conn.pcb.local_ip, C.IPADDR_TYPE_V6)
        C.ip_addr_type_set(&conn.pcb.remote_ip, C.IPADDR_TYPE_V6)
        copy(conn.pcb.local_ip.u_addr[4:], conn.remoteAddr.IP)
        copy(conn.pcb.remote_ip.u_addr[4:], conn.localAddr.IP)
    }

    err := C.udp_sendto_if_src(
        conn.pcb,
        buf,
        &conn.pcb.remote_ip,
        C.u16_t(conn.localAddr.Port),
        conn.is.netif,
        &conn.pcb.local_ip,
        C.u16_t(conn.remoteAddr.Port),
    )

    if err != C.ERR_OK {
        return 0, errors.New("write error")
    }
    return len(b), nil
}

func (conn *UDPConn) Close() error {
    // C.udp_remove(conn.pcb)
    // udp 的 pcb 的公共的
    // 不像 tcp 那样有新的 pcb 持续保留到连接关闭
    fmt.Println("Close", conn.remoteAddr)
    return nil
}

func (conn *UDPConn) LocalAddr() net.Addr {
    return conn.localAddr
}

func (conn *UDPConn) RemoteAddr() net.Addr {
    return conn.remoteAddr
}

func (conn *UDPConn) SetDeadline(t time.Time) error {
    conn.readDeadline = t
    conn.writeDeadline = t
    return nil
}

func (conn *UDPConn) SetReadDeadline(t time.Time) error {
    conn.readDeadline = t
    return nil
}

func (conn *UDPConn) SetWriteDeadline(t time.Time) error {
    conn.writeDeadline = t
    return nil
}

func newUdpConn(pcb *C.struct_udp_pcb, addr *C.ip_addr_t, port C.u16_t, destAddr *C.ip_addr_t, destPort C.u16_t, is *ipStack) *UDPConn {
    conn := &UDPConn{
        is:     is,
        pcb:    pcb,
        chRead: make(chan []byte, 3),
    }

    var remoteIp, localIp net.IP
    if C.IPADDR_TYPE_V4 == C.ip_addr_type(addr) {
        localIp = append(make([]byte, 0, 4), addr.u_addr[:4]...)
        remoteIp = append(make([]byte, 0, 4), destAddr.u_addr[:4]...)
    } else {
        localIp = append(make([]byte, 0, 16), addr.u_addr[4:]...)
        remoteIp = append(make([]byte, 0, 16), destAddr.u_addr[4:]...)
    }

    conn.localAddr = &net.UDPAddr{
        IP:   localIp,
        Port: int(port),
    }

    conn.remoteAddr = &net.UDPAddr{
        IP:   remoteIp,
        Port: int(destPort),
    }

    return conn
}
