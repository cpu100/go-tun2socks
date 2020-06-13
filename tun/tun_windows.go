package tun

import (
    "github.com/songgao/water"
    "io"
)

// type tap2tun struct {
//     tap    *water.Interface
//     rBuf   [1999]byte
//     wBuf   [1999]byte
//     inited bool
// }
//
// tap 和 tun 的区别
// tap 工作在以太层，tun 工作在 IP 层
// 前者比后者多一层 ether 头部
// https://github.com/yinghuocho/gotun2socks/blob/master/tun/tun_windows.go
//
// func (t *tap2tun) Read(bs []byte) (int, error) {
// read:
//     nr, err := t.tap.Read(t.rBuf[:])
//     if nil != err {
//         return 0, err
//     }
//     if t.rBuf[14]&0xf0 == 0x40 {
//         // log.Println("ipv4", nr)
//         if !t.inited {
//             // todo 就是源 mac 和目标 mac 对调一下，而且是固定的，不用每次都判断
//             // todo ipv6
//             // todo hyper-v 无响应可能是 mac 地址问题
//             copy(t.wBuf[:], t.rBuf[6:12])
//             copy(t.wBuf[6:], t.rBuf[0:6])
//             copy(t.wBuf[12:], t.rBuf[12:14])
//             t.inited = true
//         }
//     } else if t.rBuf[14]&0xf0 == 0x60 {
//         // log.Println("ipv6", nr)
//         goto read
//     } else {
//         log.Println("ipv?", nr)
//         goto read
//     }
//     copy(bs, t.rBuf[14:nr])
//     return nr - 14, nil
// }
//
// func (t *tap2tun) Write(bs []byte) (int, error) {
//     return t.tap.Write(append(t.wBuf[:14], bs...))
// }
//
// func (t *tap2tun) Close() error {
//     return t.tap.Close()
// }

func OpenTunDevice(name, subnet string, dnsServers string) (io.ReadWriteCloser, error) {
    cfg := water.Config{
        DeviceType: water.TUN,
        PlatformSpecificParams: water.PlatformSpecificParams{
            InterfaceName: name,
            ComponentID:   "tap0901",
            Network: subnet,
            DnsServers: dnsServers,
        },
    }

    return water.New(cfg)

    // return &tap2tun{tap: tapDev}, nil
}
