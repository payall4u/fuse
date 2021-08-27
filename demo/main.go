package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"math/rand"
	"os"
	"os/signal"
	"reflect"
	"sync"
	"syscall"
	"unsafe"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

var (
	mountPoint string
	dataFile   string
)

var hmap = sync.Map{}

func init() {
	flag.StringVar(&mountPoint, "mount", "", "moint point")
	flag.StringVar(&dataFile, "data", "", "data file")
	flag.Parse()
}

func main() {

	fs := &filesystem{
		root:     mountPoint,
		filename: dataFile,
	}
	var err error
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err = fs.Mount(); err != nil {
			fmt.Println(err.Error())
		}
	}()
	wg.Wait()
	if err != nil {
		if err = fs.Umount(); err != nil {
			fmt.Println(err.Error())
		}
	}
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt)
	<-ch
	if err = fs.Umount(); err != nil {
		fmt.Println(err.Error())
	}
}

type filesystem struct {
	root     string
	filename string
}

func (f *filesystem) Mount() error {
	conn, err := fuse.Mount(
		f.root,
		fuse.FSName("rofs"),
		fuse.ReadOnly(),
	)
	if err != nil {
		return fmt.Errorf("fuse mount failed: %s", err.Error())
	}
	if err := fs.Serve(conn, f); err != nil {
		return fmt.Errorf("server fuse failed: %s", err.Error())
	}
	return nil
}

func (f *filesystem) Umount() error {
	return fuse.Unmount(f.root)
}

func (f *filesystem) Root() (fs.Node, error) {
	info, err := os.Stat(f.filename)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(f.filename, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &dir{n: &node{
		size:     info.Size(),
		filename: f.filename,
		file:     file,
	}}, nil
}

type dir struct {
	n *node
}

func (i *dir) ReadDirAll(ctx context.Context) (res []fuse.Dirent, err error) {
	res = append(res, fuse.Dirent{
		Name:  i.n.filename,
		Type:  0,
		Inode: i.n.ino,
	})
	return
}

func (n *dir) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Inode = 1
	attr.Size = 4096
	attr.Mode = 0755 | os.ModeDir
	return nil
}

func (d *dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	return d.n, nil
}

type node struct {
	ino      uint64
	filename string
	size     int64
	file     *os.File
}

func (n *node) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Inode = n.ino
	attr.Size = uint64(n.size)
	attr.Mode = 0755
	return nil
}

func (i *node) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) (err error) {
	headerSize := int(unsafe.Sizeof(struct {
		Len    uint32
		Error  int32
		Unique uint64
	}{}))
	suffixPaddingSize := 4096 - headerSize
	allSize := 4096 + req.Size
	allBuffer := newBufferFromHeap(allSize)

	dataBuffer := allBuffer[4096:]
	ptr := (*reflect.SliceHeader)(unsafe.Pointer(&dataBuffer)).Data
	_, _, e := syscall.Syscall6(syscall.SYS_MMAP, ptr, uintptr(req.Size), uintptr(syscall.PROT_READ), uintptr(syscall.MAP_FIXED | syscall.MAP_FILE | syscall.MAP_PRIVATE), i.file.Fd(), uintptr(req.Offset))
	// size, err := i.file.ReadAt(buffer, req.Offset)
	resp.Data = allBuffer[suffixPaddingSize:]
	if int(e) != 0 {
		logrus.WithField("action", "read").Errorf("read failed: %s", e.Error())
		return e
	}
	return nil
}

func newBufferFromHeap(size int) []byte {
	m := make([]byte, size, size)
	hmap.Store(rand.Int(), m)
	return m
}