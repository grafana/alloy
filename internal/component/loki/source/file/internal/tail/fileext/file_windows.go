//go:build windows

package fileext

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// issue also described here
// https://codereview.appspot.com/8203043/

// https://github.com/jnwhiteh/golang/blob/master/src/pkg/os/file_windows.go#L133
func OpenFile(name string) (file *os.File, err error) {
	f, e := open(name, os.O_RDONLY|syscall.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(f), name), nil
}

// https://github.com/jnwhiteh/golang/blob/master/src/pkg/syscall/syscall_windows.go#L218
func open(path string, mode int, _ uint32) (fd syscall.Handle, err error) {
	if len(path) == 0 {
		return syscall.InvalidHandle, syscall.ERROR_FILE_NOT_FOUND
	}
	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	var access uint32
	switch mode & (syscall.O_RDONLY | syscall.O_WRONLY | syscall.O_RDWR) {
	case syscall.O_RDONLY:
		access = syscall.GENERIC_READ
	case syscall.O_WRONLY:
		access = syscall.GENERIC_WRITE
	case syscall.O_RDWR:
		access = syscall.GENERIC_READ | syscall.GENERIC_WRITE
	}
	if mode&syscall.O_CREAT != 0 {
		access |= syscall.GENERIC_WRITE
	}
	if mode&syscall.O_APPEND != 0 {
		access &^= syscall.GENERIC_WRITE
		access |= syscall.FILE_APPEND_DATA
	}
	sharemode := uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE | syscall.FILE_SHARE_DELETE)
	var sa *syscall.SecurityAttributes
	if mode&syscall.O_CLOEXEC == 0 {
		sa = makeInheritSa()
	}
	var createmode uint32
	switch {
	case mode&(syscall.O_CREAT|syscall.O_EXCL) == (syscall.O_CREAT | syscall.O_EXCL):
		createmode = syscall.CREATE_NEW
	case mode&(syscall.O_CREAT|syscall.O_TRUNC) == (syscall.O_CREAT | syscall.O_TRUNC):
		createmode = syscall.CREATE_ALWAYS
	case mode&syscall.O_CREAT == syscall.O_CREAT:
		createmode = syscall.OPEN_ALWAYS
	case mode&syscall.O_TRUNC == syscall.O_TRUNC:
		createmode = syscall.TRUNCATE_EXISTING
	default:
		createmode = syscall.OPEN_EXISTING
	}
	h, e := syscall.CreateFile(pathp, access, sharemode, sa, createmode, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	return h, e
}

// https://github.com/jnwhiteh/golang/blob/master/src/pkg/syscall/syscall_windows.go#L211
func makeInheritSa() *syscall.SecurityAttributes {
	var sa syscall.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	return &sa
}

func IsDeletePending(f *os.File) (bool, error) {
	if f == nil {
		return false, nil
	}

	fi, err := getFileStandardInfo(f)
	if err != nil {
		return false, err
	}

	return fi.DeletePending, nil
}

// From: https://github.com/microsoft/go-winio/blob/main/fileinfo.go
// FileStandardInfo contains extended information for the file.
// FILE_STANDARD_INFO in WinBase.h
// https://docs.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_standard_info
type fileStandardInfo struct {
	AllocationSize, EndOfFile int64
	NumberOfLinks             uint32
	DeletePending, Directory  bool
}

// GetFileStandardInfo retrieves ended information for the file.
func getFileStandardInfo(f *os.File) (*fileStandardInfo, error) {
	si := &fileStandardInfo{}
	if err := windows.GetFileInformationByHandleEx(windows.Handle(f.Fd()),
		windows.FileStandardInfo,
		(*byte)(unsafe.Pointer(si)),
		uint32(unsafe.Sizeof(*si))); err != nil {
		return nil, &os.PathError{Op: "GetFileInformationByHandleEx", Path: f.Name(), Err: err}
	}
	runtime.KeepAlive(f)
	return si, nil
}
