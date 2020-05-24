// +build windows

package ipstack

/*
#cgo CFLAGS: -I./c/include
#include "lwip/sys.h"
#include "lwip/init.h"
#include "lwip/timeouts.h"
*/
import "C"
import "time"

func init() {
	C.sys_init()  // Initialze sys_arch layer, must be called before anything else.
	C.lwip_init() // Initialze modules.
	go func() {
		for {
			C.sys_check_timeouts()
			time.Sleep(time.Millisecond * 200)
		}
	}()
}
