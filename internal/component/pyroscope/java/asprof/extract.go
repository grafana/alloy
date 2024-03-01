//go:build linux && (amd64 || arm64)

package asprof

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	//"path/filepath"

	"github.com/klauspost/compress/gzip"
	"golang.org/x/sys/unix"
)

const extractPerm = 0755

func readTarGZ(buf []byte, cb func(name string, fi fs.FileInfo, data []byte) error) error {
	gzipReader, err := gzip.NewReader(bytes.NewReader(buf))
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		fileInfo := header.FileInfo()
		if fileInfo.IsDir() {
			continue
		}
		buffer, err := io.ReadAll(tarReader)
		if err != nil {
			return err
		}
		err = cb(header.Name, fileInfo, buffer)
		if err != nil {
			return err
		}
	}

	return nil
}

func writeFile(dir *os.File, path string, data []byte, doOwnershipChecks bool) error {
	pl := strings.Split(path, string(filepath.Separator))
	it := dir
	dirPathParts := pl[:len(pl)-1]
	fname := pl[len(pl)-1]
	for _, part := range dirPathParts {
		f, err := openAt(it, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
		if err != nil {
			err = unix.Mkdirat(int(it.Fd()), part, extractPerm)
			if err != nil {
				return fmt.Errorf("failed to create directory %s %s: %w", path, part, err)
			}
			f, err = openAt(it, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
			if err != nil {
				return fmt.Errorf("failed to open directory %s %s: %w", path, part, err)
			}
		}
		defer f.Close()
		if doOwnershipChecks {
			if err = checkExtractFile(f, it); err != nil {
				return err
			}
		}
		it = f
	}
	f, err := openAt(it, fname, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return writeFileData(it, fname, path, data, doOwnershipChecks)
	}
	defer f.Close()
	if doOwnershipChecks {
		if err = checkExtractFile(f, it); err != nil {
			return err
		}
	}
	return checkFileData(f, path, data)
}

func checkFileData(f *os.File, path string, data []byte) error {
	prevData, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("failed to read file %s : %w", path, err)
	}
	if !bytes.Equal(prevData, data) {
		return fmt.Errorf("file %s already exists and is different", path)
	}
	return nil
}

func writeFileData(it *os.File, fname string, path string, data []byte, doOwnershipChecks bool) error {
	f, err := openAt(it, fname, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW, extractPerm)
	if err != nil {
		return fmt.Errorf("failed to create file %s %s: %w", path, fname, err)
	}
	defer f.Close()
	if doOwnershipChecks {
		if err = checkExtractFile(f, it); err != nil {
			return err
		}
	}
	if _, err = f.Write(data); err != nil {
		return fmt.Errorf("failed to write file %s %s: %w", path, fname, err)
	}
	return nil
}

func openAt(f *os.File, path string, flags int, mode uint32) (*os.File, error) {
	fd, err := unix.Openat(int(f.Fd()), path, flags, mode)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), filepath.Join(f.Name(), path)), nil
}

func checkTempDirPermissions(tmpDirFile *os.File) error {
	tmpDirFileStat, err := tmpDirFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat tmp dir %s: %w", tmpDirFile.Name(), err)
	}
	if !tmpDirFileStat.IsDir() {
		return fmt.Errorf("tmp dir %s is not a directory", tmpDirFile.Name())
	}
	sys := tmpDirFileStat.Sys().(*syscall.Stat_t)
	ok := false
	if sys.Uid == uint32(os.Getuid()) && tmpDirFileStat.Mode().Perm() == extractPerm {
		ok = true
	} else if sys.Uid == 0 && tmpDirFileStat.Mode()&os.ModeSticky != 0 {
		ok = true
	}
	if !ok {
		return fmt.Errorf("tmp dir %s has wrong permissions %+v", tmpDirFile.Name(), sys)
	}
	return nil
}

func checkExtractFile(f *os.File, parent *os.File) error {
	parentStat, err := parent.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", f.Name(), err)
	}
	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat  %s: %w", f.Name(), err)
	}
	sys := stat.Sys().(*syscall.Stat_t)
	parentSys := parentStat.Sys().(*syscall.Stat_t)

	ok := false
	if sys.Uid == uint32(os.Getuid()) && stat.Mode().Perm() == extractPerm {
		ok = true
	}
	if !ok {
		return fmt.Errorf("  %s has wrong permissions %+v", f.Name(), sys)
	}
	if sys.Dev != parentSys.Dev {
		return fmt.Errorf("  %s has wrong device %+v %+v", f.Name(), sys, parentSys)
	}

	actualPath, err := readlinkFD(f)
	if err != nil {
		return fmt.Errorf("failed to readlink %s: %w", f.Name(), err)
	}
	expectedPath := f.Name()
	if actualPath != expectedPath {
		return fmt.Errorf("expected %s, but it is %s", expectedPath, actualPath)
	}
	return nil
}

func readlinkFD(f *os.File) (string, error) {
	return os.Readlink(fmt.Sprintf("/proc/self/fd/%d", f.Fd()))
}
