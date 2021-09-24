package pipes

import (
	"golang.org/x/sys/unix"
	"syscall"
)

func init() {
	for i := 0; i < 10; i++ {
		r, w, e := NewPipe()
		if e != nil {
			panic(e)
		}
		Pipes[i] = Wrapper{
			R: r,
			W: w,
		}
	}
}

var Pipes [10]Wrapper

type Wrapper struct {
	R int
	W int
}

func NewPipe() (int, int, error) {
	var p [2]int

	e := syscall.Pipe2(p[0:], syscall.O_CLOEXEC | syscall.O_NONBLOCK)
	// pipe2 was added in 2.6.27 and our minimum requirement is 2.6.23, so it
	// might not be implemented.
	if e == syscall.ENOSYS {
		// See ../syscall/exec.go for description of lock.
		syscall.ForkLock.RLock()
		e = syscall.Pipe(p[0:])
		if e != nil {
			syscall.ForkLock.RUnlock()
			return -1, -1, e
		}
		syscall.CloseOnExec(p[0])
		syscall.CloseOnExec(p[1])
		syscall.ForkLock.RUnlock()
	} else if e != nil {
		return -1, -1, e
	}
	if _, e := unix.FcntlInt(uintptr(p[0]), unix.F_SETPIPE_SZ, 16384 * 4 + 4096); e != nil {
		panic(e)
	}
	return p[0], p[1], nil
}