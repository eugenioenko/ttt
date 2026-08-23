//go:build linux || darwin || freebsd

package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func readWorkingTreeFilePlatform(ctx context.Context, root, path string) (workingTreeFile, error) {
	dirfd, err := openDirectoryPathNoFollow(ctx, root)
	if err != nil {
		return workingTreeFile{}, err
	}
	defer func() { _ = unix.Close(dirfd) }()

	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	for _, part := range parts[:len(parts)-1] {
		if err := ctx.Err(); err != nil {
			return workingTreeFile{}, err
		}
		next, openErr := unix.Openat(dirfd, part, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
		if openErr != nil {
			return workingTreeFile{}, classifyWorkingTreeOpenError(filepath.Join(root, path), workingTreePathSymlinkComponent, openErr)
		}
		if closeErr := unix.Close(dirfd); closeErr != nil {
			unix.Close(next)
			return workingTreeFile{}, closeErr
		}
		dirfd = next
	}

	return readWorkingTreeFinalAt(ctx, dirfd, parts[len(parts)-1], filepath.Join(root, path))
}

func openDirectoryPathNoFollow(ctx context.Context, path string) (int, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return -1, &workingTreePathError{Path: path, Kind: workingTreePathUnverified, Err: err}
	}
	if abs == string(filepath.Separator) {
		return unix.Open(abs, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return -1, &workingTreePathError{Path: path, Kind: workingTreePathUnverified, Err: err}
	}
	fd, err := openAbsoluteDirectoryNoFollow(ctx, parent, path)
	if err != nil {
		return -1, err
	}
	next, openErr := unix.Openat(fd, filepath.Base(abs), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	closeErr := unix.Close(fd)
	if openErr != nil {
		return -1, classifyWorkingTreeOpenError(path, workingTreePathSymlinkComponent, openErr)
	}
	if closeErr != nil {
		unix.Close(next)
		return -1, closeErr
	}
	return next, nil
}

func openAbsoluteDirectoryNoFollow(ctx context.Context, abs, reportedPath string) (int, error) {
	fd, err := unix.Open(string(filepath.Separator), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return -1, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(filepath.Clean(abs), string(filepath.Separator)), string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		if err := ctx.Err(); err != nil {
			unix.Close(fd)
			return -1, err
		}
		next, openErr := unix.Openat(fd, part, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
		if openErr != nil {
			unix.Close(fd)
			return -1, classifyWorkingTreeOpenError(reportedPath, workingTreePathSymlinkComponent, openErr)
		}
		if closeErr := unix.Close(fd); closeErr != nil {
			unix.Close(next)
			return -1, closeErr
		}
		fd = next
	}
	return fd, nil
}

func readWorkingTreeFinalAt(ctx context.Context, dirfd int, name, path string) (workingTreeFile, error) {
	for range 4 {
		if err := ctx.Err(); err != nil {
			return workingTreeFile{}, err
		}
		var stat unix.Stat_t
		if err := unix.Fstatat(dirfd, name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			if errors.Is(err, unix.ENOENT) {
				return workingTreeFile{Kind: workingTreeFileMissing}, nil
			}
			return workingTreeFile{}, err
		}
		switch stat.Mode & unix.S_IFMT {
		case unix.S_IFLNK:
			content, err := readlinkAtContext(ctx, dirfd, name)
			if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOENT) {
				continue
			}
			if err != nil {
				return workingTreeFile{}, err
			}
			return workingTreeFile{Content: content, Exists: true, Kind: workingTreeFileSymlink}, nil
		case unix.S_IFREG:
			fd, err := unix.Openat(dirfd, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
			if errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOENT) {
				continue
			}
			if err != nil {
				return workingTreeFile{}, err
			}
			content, readErr := readRegularFileContext(ctx, fd, path)
			closeErr := unix.Close(fd)
			if readErr != nil {
				return workingTreeFile{}, readErr
			}
			if closeErr != nil {
				return workingTreeFile{}, closeErr
			}
			return workingTreeFile{Content: content, Exists: true, Kind: workingTreeFileRegular}, nil
		case unix.S_IFDIR:
			return workingTreeFile{}, &workingTreePathError{Path: path, Kind: workingTreePathDirectory}
		case unix.S_IFIFO:
			return workingTreeFile{}, &workingTreePathError{Path: path, Kind: workingTreePathFIFO}
		case unix.S_IFSOCK:
			return workingTreeFile{}, &workingTreePathError{Path: path, Kind: workingTreePathSocket}
		case unix.S_IFCHR, unix.S_IFBLK:
			return workingTreeFile{}, &workingTreePathError{Path: path, Kind: workingTreePathDevice}
		default:
			return workingTreeFile{}, &workingTreePathError{Path: path, Kind: workingTreePathSpecial}
		}
	}
	return workingTreeFile{}, &workingTreePathError{Path: path, Kind: workingTreePathUnverified, Err: errors.New("path identity changed during read")}
}

func readRegularFileContext(ctx context.Context, fd int, path string) ([]byte, error) {
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, &workingTreePathError{Path: path, Kind: workingTreePathSpecial, Err: errors.New("opened object is not a regular file")}
	}
	var content bytes.Buffer
	if stat.Size > 0 && stat.Size <= 8<<20 {
		content.Grow(int(stat.Size))
	}
	buf := make([]byte, workingTreeReadChunk)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, err := unix.Read(fd, buf)
		if n > 0 {
			_, _ = content.Write(buf[:n])
		}
		if n == 0 && err == nil {
			return content.Bytes(), nil
		}
		if err == nil {
			continue
		}
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if errors.Is(err, unix.EAGAIN) {
			continue
		}
		if errors.Is(err, io.EOF) {
			return content.Bytes(), nil
		}
		return nil, err
	}
}

func readlinkAtContext(ctx context.Context, dirfd int, name string) ([]byte, error) {
	for size := 256; size <= 64*1024; size *= 2 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		buf := make([]byte, size)
		n, err := unix.Readlinkat(dirfd, name, buf)
		if err != nil {
			return nil, err
		}
		if n < len(buf) {
			return buf[:n], nil
		}
	}
	return nil, &workingTreePathError{Path: name, Kind: workingTreePathUnverified, Err: errors.New("symlink target is too long")}
}

func classifyWorkingTreeOpenError(path string, kind workingTreePathKind, err error) error {
	if errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) {
		return &workingTreePathError{Path: path, Kind: kind, Err: err}
	}
	return err
}
