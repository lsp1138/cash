// Copyright (c) 2014 The fileutil Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !arm && !arm64
// +build !arm,!arm64

package fileutil // import "modernc.org/fileutil"

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"
	"syscall"
)

const hasPunchHole = true

func getKernelVersion() ([]byte, error) {
	tryGetRawVersion := func() ([]byte, error) {
		// 1: Try /proc/sys/kernel/osrelease
		if b, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
			return bytes.TrimSpace(b), nil
		}

		// 2: Try /proc/version
		if b, err := os.ReadFile("/proc/version"); err == nil {
			// Parse "Linux version X.Y.Z-..."
			fields := bytes.Fields(b)
			if len(fields) >= 3 && bytes.Equal(fields[0], []byte("Linux")) && bytes.Equal(fields[1], []byte("version")) {
				version := fields[2]
				// Remove any suffix after the first non-version character
				for i, c := range version {
					if c != '.' && (c < '0' || c > '9') {
						version = version[:i]
						break
					}
				}
				return version, nil
			}
		}

		// 3: Try uname syscall
		var utsname syscall.Utsname
		if err := syscall.Uname(&utsname); err == nil {
			// Convert [65]int8 to []byte and find null terminator
			release := make([]byte, 0, 65)
			for _, c := range utsname.Release {
				if c == 0 {
					break
				}
				release = append(release, byte(c))
			}
			if len(release) > 0 {
				return release, nil
			}
		}

		// If all methods fail, return an error
		return nil, errors.New("failed to determine kernel version")
	}

	n := func(s []byte) byte {
		for i, c := range s {
			if c < '0' || c > '9' {
				s = s[:i]
				break
			}
		}
		v, _ := strconv.Atoi(string(s))
		return byte(v)
	}

	b, err := tryGetRawVersion()
	if err != nil {
		return nil, err
	}

	tokens := bytes.Split(b, []byte("."))
	if len(tokens) > 3 {
		tokens = tokens[:3]
	}

	// Ensure all tokens are numeric and convert to bytes
	var ret = make([]byte, len(tokens))
	for i := range tokens {
		ret[i] = n(tokens[i])
	}
	return ret, nil
}

func init() {
	b, err := getKernelVersion()
	if err != nil {
		panic(err)
	}

	switch len(b) {
	case 3:
		// Supported since kernel 2.6.38
		if bytes.Compare(b, []byte{2, 6, 38}) < 0 {
			puncher = func(*os.File, int64, int64) error { return nil }
		}
	case 2:
		if bytes.Compare(b, []byte{2, 7}) < 0 {
			puncher = func(*os.File, int64, int64) error { return nil }
		}
	default:
		puncher = func(*os.File, int64, int64) error { return nil }
	}
}

var puncher = func(f *os.File, off, len int64) error {
	const (
		/*
			/usr/include/linux$ grep FL_ falloc.h
		*/
		_FALLOC_FL_KEEP_SIZE  = 0x01 // default is extend size
		_FALLOC_FL_PUNCH_HOLE = 0x02 // de-allocates range
	)

	_, _, errno := syscall.Syscall6(
		syscall.SYS_FALLOCATE,
		uintptr(f.Fd()),
		uintptr(_FALLOC_FL_KEEP_SIZE|_FALLOC_FL_PUNCH_HOLE),
		uintptr(off),
		uintptr(len),
		0, 0)
	if errno != 0 {
		return os.NewSyscallError("SYS_FALLOCATE", errno)
	}
	return nil
}

// PunchHole deallocates space inside a file in the byte range starting at
// offset and continuing for len bytes. No-op for kernels < 2.6.38 (or < 2.7).
func PunchHole(f *os.File, off, len int64) error {
	return puncher(f, off, len)
}

// Fadvise predeclares an access pattern for file data.  See also 'man 2
// posix_fadvise'.
func Fadvise(f *os.File, off, len int64, advice FadviseAdvice) error {
	_, _, errno := syscall.Syscall6(
		syscall.SYS_FADVISE64,
		uintptr(f.Fd()),
		uintptr(off),
		uintptr(len),
		uintptr(advice),
		0, 0)
	return os.NewSyscallError("SYS_FADVISE64", errno)
}

// IsEOF reports whether err is an EOF condition.
func IsEOF(err error) bool { return err == io.EOF }
