package main

// #cgo LDFLAGS: -lrpm -lrpmio
// #include <rpm/rpmts.h>
// #include <rpm/rpmlib.h>
// #include <rpm/header.h>
// #include <rpm/rpmio.h>
import "C"

import "unsafe"
import "errors"
import "fmt"

type rpmts C.rpmts

func newTS() rpmts {
	ts := C.rpmtsCreate()
	// remove some checking to prevent rpmReadPackageFile() from printing warning messages
	C.rpmtsSetVSFlags(ts, C._RPMVSF_NOSIGNATURES|C._RPMVSF_NODIGESTS|C.RPMVSF_NOHDRCHK)
	return rpmts(ts)
}

func freeTS(ts rpmts) {
	C.rpmtsFree(ts)
}

type rpmheader struct {
	header C.Header
}

func openRPM(ts rpmts, path string) (*rpmheader, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	cMode := C.CString("r")
	defer C.free(unsafe.Pointer(cMode))
	var header rpmheader

	fd := C.Fopen(cPath, cMode)
	// FIXME: handle error
	rc := C.rpmReadPackageFile(ts, fd, nil, &header.header)
	C.Fclose(fd)
	switch rc {
	default:
		return nil, errors.New("Parse package '" + path + "' failed!")
	case C.RPMRC_OK, C.RPMRC_NOKEY:
	}
	return &header, nil
}

func (header *rpmheader) close() {
	C.headerFree(header.header)
	header.header = nil
}

type rpmtag C.rpmTagVal

const (
	rpmtagName rpmtag = rpmtag(C.RPMTAG_NAME)
)

func (header *rpmheader) getString(tag rpmtag) (string, error) {
	cStr := C.headerGetString(header.header, C.rpmTagVal(tag))
	if cStr == nil {
		return "", errors.New(fmt.Sprintf("failed to get value of tag(%d) as string.", tag))
	}
	return C.GoString(cStr), nil
}
