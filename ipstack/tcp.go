package ipstack

/*
#include "c/include/lwip/tcp.h"
#include "c/custom/tool/tool.h"

extern err_t tcpRecvFn(void *arg, struct tcp_pcb *tpcb, struct pbuf *p, err_t err);
extern err_t tcpSentFn(void *arg, struct tcp_pcb *tpcb, u16_t len);
extern void tcpErrFn(void *arg, err_t err);
extern err_t tcpPollFn(void *arg, struct tcp_pcb *tpcb);

*/
import "C"
import (
    "errors"
    "fmt"
    "io"
    "log"
    "net"
    "runtime"
    "sync"
    "time"
    "unsafe"
)

type TCPConn struct {
    localAddr  *net.TCPAddr
    remoteAddr *net.TCPAddr

    chRead chan *C.struct_pbuf

    condSent *sync.Cond

    rTimer    *time.Timer
    rDuration time.Duration

    is  *ipStack
    pcb *C.struct_tcp_pcb
}

func (conn *TCPConn) WriteTo(w io.Writer) (n int64, err error) {
    var p *C.struct_pbuf
    for {
        select {
        case p = <-conn.chRead:
            if nil == p {
                log.Println("nil == p")
                return
            }
        case <-conn.rTimer.C:
            poolPutTimer(conn.rTimer, false)
            err = ErrTimeout
            log.Println(err, conn.localAddr, conn.remoteAddr)
            return
        }
        log.Println("read", p.len)
        if nil == conn.pcb {
            log.Println("w.Write: nil == conn.pcb")
            return
        }
        for {
            buf := (*[1 << 30]byte)(unsafe.Pointer(p.payload))[:p.len:p.len]
            nw, err2 := w.Write(buf)
            n += int64(nw)
            if nil != err2 {
                // 暂不处理临时错误的情况
                C.tcp_abort(conn.pcb)
                err = errors.New("write to: " + err2.Error())
                log.Println(err)
                return
            }
            if len(buf) != nw {
                C.tcp_abort(conn.pcb)
                err = io.ErrShortWrite
                log.Println(err)
                return
            }
            r := p
            p = p.next
            conn.is.Lock()
            r.next = nil
            C.tcp_recved(conn.pcb, r.len)
            C.pbuf_free(r)
            conn.is.Unlock()
            if nil == p {
                break
            } else {
                // todo 构造测试数据，包括 udp
                fmt.Println("debug: p.next")
            }
        }
    }
}

func (conn *TCPConn) ReadFrom(r io.Reader) (n int64, err error) {
    conn.condSent.L.Lock()
    defer conn.condSent.L.Unlock()

    b := make([]byte, 1500)
    for {
        nr, err2 := r.Read(b)
        if nil != err2 {
            if err2 != io.EOF {
                err = err2
            }
            break
        }
        if nil == conn.pcb {
            log.Println("tcp_write: nil == conn.pcb")
            break
        }
        conn.is.Lock()
        errno := C.tcp_write(conn.pcb, unsafe.Pointer(&b[0]), C.u16_t(nr), C.TCP_WRITE_FLAG_COPY)
        conn.is.Unlock()
        if errno == C.ERR_OK {

            conn.is.Lock()
            C.tcp_output(conn.pcb)
            conn.is.Unlock()

            conn.condSent.Wait()

            n += int64(nr)
            continue
        } else {
            C.tcp_abort(conn.pcb)
            err = errors.New("write error")
            log.Println(err)
            break
        }
    }
    return
}

func (conn *TCPConn) Read(b []byte) (int, error) {
    panic("implement me")
}

func (conn *TCPConn) Write(b []byte) (int, error) {
    panic("implement me")
    // conn.is.Lock()
    // defer conn.is.Unlock()
    // err := C.tcp_write(conn.pcb, unsafe.Pointer(&b[0]), C.u16_t(len(b)), C.TCP_WRITE_FLAG_COPY)
    // if err == C.ERR_OK {
    //     C.tcp_output(conn.pcb)
    //     return len(b), nil
    // } else if err == C.ERR_MEM {
    //     return 0, errors.New("Write Error")
    // }
    // return 0, errors.New("Write Error")
}

func (conn *TCPConn) Close() error {

    conn.is.Lock()
    defer conn.is.Unlock()
    fmt.Println("close tcp", conn.localAddr, conn.remoteAddr)

    if nil == conn.pcb {
        return ErrCloseClosed
    }

    // C.tcp_close(conn.pcb)
    // C.tcp_shutdown(conn.pcb, 1, 1)
    // 重复关闭，或者在不恰当的地方关闭，会导致访问冲突错误
    // Process finished with exit code -1073740940 (0xC0000374)
    conn.pcb = nil

    close(conn.chRead)
    for {
        p := <-conn.chRead
        if nil == p {
            break
        }
        C.pbuf_free(p)
    }

    return nil
}

func (conn *TCPConn) LocalAddr() net.Addr {
    return conn.localAddr
}

func (conn *TCPConn) RemoteAddr() net.Addr {
    return conn.remoteAddr
}

func (conn *TCPConn) SetDeadline(t time.Time) error {
    panic("implement me")
}

func (conn *TCPConn) SetReadDeadline(t time.Time) error {
    conn.rDuration = t.Sub(time.Now())
    if nil != conn.rTimer {
        poolPutTimer(conn.rTimer, true)
    }
    conn.rTimer = poolGetTimer(conn.rDuration)
    return nil
}

func (conn *TCPConn) SetWriteDeadline(t time.Time) error {
    panic("implement me")
}

func newTcpConn(pcb *C.struct_tcp_pcb, is *ipStack) *TCPConn {
    conn := &TCPConn{
        is:       is,
        pcb:      pcb,
        chRead:   make(chan *C.struct_pbuf, 8),
        condSent: sync.NewCond(new(sync.Mutex)),
    }

    // (*TCPConn).Close 等同 func Close (*TCPConn) { } 即第一个参数是接收器
    runtime.SetFinalizer(conn, (*TCPConn).Close)

    is.Lock()
    defer is.Unlock()
    C.tcp_arg_cgo(pcb, C.uintptr_t(uintptr(unsafe.Pointer(conn))))
    C.tcp_recv(pcb, Cptr(C.tcpRecvFn))
    C.tcp_sent(pcb, Cptr(C.tcpSentFn))
    C.tcp_err(pcb, Cptr(C.tcpErrFn))
    C.tcp_poll(pcb, Cptr(C.tcpPollFn), C.u8_t(8))

    var remoteIp, localIp net.IP
    if C.IPADDR_TYPE_V4 == C.ip_addr_type(&pcb.local_ip) {
        // 在 go 中用不了 c union 成员名字，但能以 slice 形式访问
        // c union 在内存中存储的顺序和它们的名字顺序相反
        localIp = pcb.remote_ip.u_addr[:4]
        remoteIp = pcb.local_ip.u_addr[:4]
    } else {
        localIp = pcb.remote_ip.u_addr[4:]
        remoteIp = pcb.local_ip.u_addr[4:]
    }

    conn.localAddr = &net.TCPAddr{
        IP:   localIp,
        Port: int(pcb.remote_port),
    }

    conn.remoteAddr = &net.TCPAddr{
        IP:   remoteIp,
        Port: int(pcb.local_port),
    }

    return conn
}
