package main

import (
	"bazil.org/fuse/pipes"
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"unsafe"

	_ "net/http/pprof"

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
	pipes.Name = dataFile
	go func() {
		http.ListenAndServe("localhost:8899", nil)
	}()
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
	header := newHeaderFromHeap()
	resp.Header = unsafe.Pointer(header)
	resp.Fd = int(i.file.Fd())
	resp.Offset = req.Offset
	resp.BufferLen = req.Size
	return nil
}

type outHeader struct {
	Len    uint32
	Error  int32
	Unique uint64
}

func newBufferFromHeap(size int) []byte {
	size = (size + (4096 - 1)) / 4096 * 4096
	m := make([]byte, size, size)
	hmap.Store(rand.Int(), m)
	return m
}

func newHeaderFromHeap() *outHeader {
	s := &outHeader{}
	hmap.Store(rand.Int(), s)
	return s
}
