// +build linux darwin

package ipstack

/*
#cgo CFLAGS: -I./c/include
#include "lwip/init.h"
#include "lwip/timeouts.h"
*/
import "C"
import "time"

func init() {
	C.lwip_init() // Initialze modules.
	go func() {
		for {
			C.sys_check_timeouts()
			time.Sleep(time.Millisecond * 200)
		}
	}()
}
