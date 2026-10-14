//go:build windows

package dirlink

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type reparseBufferHeader struct {
	ReparseTag        uint32
	ReparseDataLength uint16
	Reserved          uint16
}

type reparseDataBuffer struct {
	header reparseBufferHeader
	detail [windows.MAXIMUM_REPARSE_DATA_BUFFER_SIZE]byte
}

type mountPointReparseBuffer struct {
	SubstituteNameOffset uint16
	SubstituteNameLength uint16
	PrintNameOffset      uint16
	PrintNameLength      uint16
	PathBuffer           [1]uint16
}

func reparseTag(path string) (uint32, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	h, err := windows.CreateFile(
		pathPtr,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(h)

	rdbbuf := make([]byte, windows.MAXIMUM_REPARSE_DATA_BUFFER_SIZE)
	var bytesReturned uint32
	err = windows.DeviceIoControl(
		h,
		windows.FSCTL_GET_REPARSE_POINT,
		nil,
		0,
		&rdbbuf[0],
		uint32(len(rdbbuf)),
		&bytesReturned,
		nil,
	)
	if err != nil {
		return 0, err
	}
	rdb := (*reparseBufferHeader)(unsafe.Pointer(&rdbbuf[0]))
	return rdb.ReparseTag, nil
}

func linkKindFromTag(tag uint32) LinkKind {
	switch tag {
	case windows.IO_REPARSE_TAG_SYMLINK:
		return KindSymlink
	case windows.IO_REPARSE_TAG_MOUNT_POINT:
		return KindJunction
	default:
		return KindNone
	}
}

func setReparsePoint(linkPath string, rdb *reparseDataBuffer) error {
	if err := windows.Mkdir(linkPath, 0o777); err != nil {
		return err
	}

	linkPtr, err := syscall.UTF16PtrFromString(linkPath)
	if err != nil {
		return err
	}
	fd, err := syscall.CreateFile(
		linkPtr,
		syscall.GENERIC_WRITE,
		0,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_OPEN_REPARSE_POINT|syscall.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(fd)

	buflen := uint32(rdb.header.ReparseDataLength) + uint32(unsafe.Sizeof(rdb.header))
	var bytesReturned uint32
	return windows.DeviceIoControl(
		windows.Handle(fd),
		windows.FSCTL_SET_REPARSE_POINT,
		(*byte)(unsafe.Pointer(&rdb.header)),
		buflen,
		nil,
		0,
		&bytesReturned,
		nil,
	)
}

type mountPointTarget struct {
	substituteName struct {
		offset, length uint16
	}
	printName struct {
		offset, length uint16
	}
	pathBuf []uint16
}

func (m *mountPointTarget) addString(s string) (offset, length uint16) {
	p, err := syscall.UTF16FromString(s)
	if err != nil {
		panic(err)
	}
	off := len(m.pathBuf) * 2
	m.pathBuf = append(m.pathBuf, p...)
	return uint16(off), uint16(len(p)-1) * 2
}

func (m *mountPointTarget) addSubstituteName(name string) {
	m.substituteName.offset, m.substituteName.length = m.addString(name)
}

func (m *mountPointTarget) addPrintName(name string) {
	m.printName.offset, m.printName.length = m.addString(name)
}

func (m *mountPointTarget) pathBufferLenBytes() uint16 {
	return uint16(len(m.pathBuf)) * 2
}

func createJunction(linkPath, absTarget string) error {
	var target mountPointTarget
	target.addSubstituteName(`\??\` + absTarget)
	target.addPrintName(absTarget)

	var buf *mountPointReparseBuffer
	bufsize := int(unsafe.Offsetof(buf.PathBuffer)) + len(target.pathBuf)*2
	if bufsize > windows.MAXIMUM_REPARSE_DATA_BUFFER_SIZE-int(unsafe.Sizeof(reparseBufferHeader{})) {
		return fmt.Errorf("junction target exceeds Windows reparse buffer limit")
	}
	buflen := uint16(bufsize)
	byteblob := make([]byte, buflen)
	buf = (*mountPointReparseBuffer)(unsafe.Pointer(&byteblob[0]))
	buf.SubstituteNameOffset = target.substituteName.offset
	buf.SubstituteNameLength = target.substituteName.length
	buf.PrintNameOffset = target.printName.offset
	buf.PrintNameLength = target.printName.length
	pbuflen := len(target.pathBuf)
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(&buf.PathBuffer[0])), pbuflen), target.pathBuf)

	var rdb reparseDataBuffer
	rdb.header.ReparseTag = windows.IO_REPARSE_TAG_MOUNT_POINT
	rdb.header.ReparseDataLength = buflen
	copy(rdb.detail[:], byteblob)

	if err := setReparsePoint(linkPath, &rdb); err != nil {
		_ = os.Remove(linkPath)
		return fmt.Errorf("create directory junction: %w", err)
	}
	return nil
}

const (
	symbolicLinkAllowUnprivilegedCreate = 0x2
	symlinkFlagRelative                 = 0x1
)

func createDirectorySymlink(linkPath, target string, relative bool) error {
	linkPtr, err := windows.UTF16PtrFromString(linkPath)
	if err != nil {
		return err
	}
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	flags := uint32(windows.SYMBOLIC_LINK_FLAG_DIRECTORY | symbolicLinkAllowUnprivilegedCreate)
	if relative {
		flags |= symlinkFlagRelative
	}
	if err := windows.CreateSymbolicLink(linkPtr, targetPtr, flags); err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno == syscall.ERROR_PRIVILEGE_NOT_HELD {
			return fmt.Errorf("create directory symlink to %s: Windows blocked unprivileged symlink creation (enable Developer Mode or grant Create symbolic links); %w", target, err)
		}
		return fmt.Errorf("create directory symlink to %s: %w", target, err)
	}
	return nil
}
