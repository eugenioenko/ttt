package trash

import (
	"context"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	createFileOperation = windows.NewLazySystemDLL("ole32.dll").NewProc("CoCreateInstance")
	createShellItem     = windows.NewLazySystemDLL("shell32.dll").NewProc("SHCreateItemFromParsingName")
	classFileOperation  = windows.GUID{Data1: 0x3ad05575, Data2: 0x8857, Data3: 0x4850, Data4: [8]byte{0x92, 0x77, 0x11, 0xb8, 0x5b, 0xdb, 0x8e, 0x09}}
	iidFileOperation    = windows.GUID{Data1: 0x947aab5f, Data2: 0x0a5c, Data3: 0x4c13, Data4: [8]byte{0xb4, 0xd6, 0x4b, 0xf7, 0x83, 0x6f, 0xc9, 0xf8}}
	iidShellItem        = windows.GUID{Data1: 0x43826d1e, Data2: 0xe718, Data3: 0x42ee, Data4: [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe}}
)

type shellObject struct{ vtable *[23]uintptr }

//go:uintptrescapes
func (o *shellObject) call(index int, args ...uintptr) error {
	result, _, _ := syscall.SyscallN(o.vtable[index], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return checkHRESULT(result)
}

func checkHRESULT(result uintptr) error {
	if int32(result) < 0 {
		return fmt.Errorf("Windows Shell HRESULT 0x%08X", uint32(result))
	}
	return nil
}

func move(ctx context.Context, path string) error {
	// COM apartments belong to OS threads. Do not change the caller's apartment.
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		done <- recycle(ctx, path)
	}()
	// The Shell owns an in-flight operation; cancellation stops waiting for it.
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func recycle(ctx context.Context, path string) error {
	const apartmentThreaded = 2
	if err := windows.CoInitializeEx(0, apartmentThreaded); err != nil && err != windows.Errno(1) {
		return err
	}
	defer windows.CoUninitialize()
	var operation *shellObject
	result, _, _ := createFileOperation.Call(uintptr(unsafe.Pointer(&classFileOperation)), 0, 1,
		uintptr(unsafe.Pointer(&iidFileOperation)), uintptr(unsafe.Pointer(&operation)))
	if err := checkHRESULT(result); err != nil {
		return err
	}
	defer operation.call(2) // IUnknown.Release
	const (
		fofSilent              = 0x0004
		fofNoConfirmation      = 0x0010
		fofNoErrorUI           = 0x0400
		fofNoConnectedElements = 0x2000
		fofWantNukeWarning     = 0x4000
		fofxRecycleOnDelete    = 0x00080000
		fofxEarlyFailure       = 0x00100000
	)
	// Refuse permanent deletion when a volume cannot recycle the item.
	flags := uintptr(fofSilent | fofNoConfirmation | fofNoErrorUI | fofNoConnectedElements | fofWantNukeWarning | fofxRecycleOnDelete | fofxEarlyFailure)
	if err := operation.call(5, flags); err != nil { // SetOperationFlags
		return err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var item *shellObject
	result, _, _ = createShellItem.Call(uintptr(unsafe.Pointer(name)), 0,
		uintptr(unsafe.Pointer(&iidShellItem)), uintptr(unsafe.Pointer(&item)))
	if err := checkHRESULT(result); err != nil {
		return err
	}
	defer item.call(2)
	if err := operation.call(18, uintptr(unsafe.Pointer(item)), 0); err != nil { // DeleteItem
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := operation.call(21); err != nil { // PerformOperations
		return err
	}
	var aborted int32
	if err := operation.call(22, uintptr(unsafe.Pointer(&aborted))); err != nil { // GetAnyOperationsAborted
		return err
	}
	if aborted != 0 {
		return fmt.Errorf("Windows recycling was aborted")
	}
	return nil
}
