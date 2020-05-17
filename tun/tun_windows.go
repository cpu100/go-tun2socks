package tun

import (
    "encoding/binary"
    "github.com/songgao/water"
    "io"
    "net"
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

func OpenTunDevice(name, addr, gw, mask string, dnsServers []string, persist bool) (io.ReadWriteCloser, error) {
    cfg := water.Config{
        DeviceType: water.TUN,
        PlatformSpecificParams: water.PlatformSpecificParams{
            InterfaceName: name,
            ComponentID:   "tap0901",
        },
    }

    tunDev, err := water.New(cfg)
    if err != nil {
        return nil, err
    }

    // 如果适配器选项没有配置为 DHCP 自动获取，以下指令将不会生效

    // set addr with dhcp
    buffer := make([]byte, 4*4)
    copy(buffer[0:], net.ParseIP(addr).To4())
    copy(buffer[4:], net.ParseIP(mask).To4())
    copy(buffer[8:], net.ParseIP(gw).To4())
    // lease, 即租期(DHCP概念)
    binary.BigEndian.PutUint32(buffer[12:], 60*60*24*365)
    err = water.DeviceIoControl(7, buffer)
    if err != nil {
        tunDev.Close()
        return nil, err
    }

    // 	primaryDNS := net.ParseIP(dnsServers[0]).To4()
    // 	dnsParam := append([]byte{6, 4}, primaryDNS...)
    // 	if len(dnsServers) >= 2 {
    // 		secondaryDNS := net.ParseIP(dnsServers[1]).To4()
    // 		dnsParam = append(dnsParam, secondaryDNS...)
    // 		dnsParam[1] += 4
    // 	}

    // set dns with dhcp
    buffer = make([]byte, 2+4)
    buffer[0] = 6
    buffer[1] = 4
    copy(buffer[2:], net.ParseIP(dnsServers[0]).To4())
    err = water.DeviceIoControl(9, buffer)
    if err != nil {
        tunDev.Close()
        return nil, err
    }

    // set tun (default tap) mode
    // tun 模式下不用处理 MAC 地址
    buffer = make([]byte, 0, 12)
    ipv4 := net.ParseIP(addr).To4()
    buffer = append(buffer, ipv4...)
    buffer = append(buffer, ipv4[0], ipv4[1], ipv4[2], 0)
    buffer = append(buffer, net.ParseIP(mask).To4()...)
    err = water.DeviceIoControl(10, buffer)
    if err != nil {
        tunDev.Close()
        return nil, err
    }

    // set media status
    buffer = []byte{1, 0, 0, 0}
    err = water.DeviceIoControl(6, buffer)
    if err != nil {
        tunDev.Close()
        return nil, err
    }

    // return &tap2tun{tap: tapDev}, nil
    return tunDev, nil
}
