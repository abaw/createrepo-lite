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

type rpmts struct {
	ts C.rpmts
}

// newTS allocates a rpmts object which is needed for most of RPM related functions
func newTS() rpmts {
	ts := C.rpmtsCreate()
	// remove some checking to prevent rpmReadPackageFile() from printing warning messages
	C.rpmtsSetVSFlags(ts, C._RPMVSF_NOSIGNATURES|C._RPMVSF_NODIGESTS|C.RPMVSF_NOHDRCHK)
	return rpmts{ts}
}

// close deallocates the rpmts object allocated by newTS
func (ts rpmts) close() {
	C.rpmtsFree(ts.ts)
}

type rpmheader struct {
	header C.Header
	path string
}

// openRPM open a RPM file and returns a rpmheader object where you could do various RPM related operations on.
func (ts rpmts) openRPM(path string) (*rpmheader, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	cMode := C.CString("r")
	defer C.free(unsafe.Pointer(cMode))
	header := rpmheader{ path: path }

	fd := C.Fopen(cPath, cMode)
	// FIXME: handle error
	rc := C.rpmReadPackageFile(ts.ts, fd, nil, &header.header)
	C.Fclose(fd)
	switch rc {
	default:
		return nil, errors.New("Parse package '" + path + "' failed!")
	case C.RPMRC_OK, C.RPMRC_NOKEY:
	}
	return &header, nil
}

// close free the resources in a rpmheader object
func (header *rpmheader) close() {
	C.headerFree(header.header)
	header.header = nil
}

type rpmtag struct {
	header *rpmheader
	name string
	value C.rpmTagVal
}

// getTag returns a rpmtag object for the given tag name
func (header *rpmheader) getTag(tag string) (rpmtag, error) {
	cTag := C.CString(tag)
	defer C.free(unsafe.Pointer(cTag))

	tagVal := C.rpmTagGetValue(cTag)
	if tagVal == 0 {
		return rpmtag{}, errors.New(fmt.Sprintf("unknown rpm tag: %s", tag))
	}

	return rpmtag{header: header, name: tag, value: tagVal}, nil

}

// getString returns the value in the tag as String.
func (tag rpmtag) getString() (string, error) {
	cStr := C.headerGetString(tag.header.header, tag.value)
	if cStr == nil {
		return "", errors.New(fmt.Sprintf("failed to get value of tag(%s) as string.", tag.name))
	}
	return C.GoString(cStr), nil
}

// getNumber returns the value in the tag as a number
func (tag rpmtag) getNumber() (uint64, error) {
	var td C.struct_rpmtd_s

	ret := C.headerGet(tag.header.header, tag.value, &td, C.HEADERGET_EXT)
	if ret != 0 || C.rpmtdCount(&td) != 1 {
		return 0, errors.New(fmt.Sprintf("not found tag(%s) in header.", tag.name))
	}

	return uint64(C.rpmtdGetNumber(&td)), nil
}

// getString returns the value of given tag in the RPM header as string
func (header *rpmheader) getString(tagName string) (string, error) {
	tag, err := header.getTag(tagName)
	if err != nil {
		return "", err
	}

	return tag.getString()
}

// getNumber returns the value of the given tag in the RPM header as a number
func (header *rpmheader) getNumber(tagName string) (uint64, error) {
	tag, err := header.getTag(tagName)
	if err != nil {
		return 0, err
	}

	return tag.getNumber()
}

// getHeaderRange return the byte range of the header in the RPM file as (startOffset, endOffset)
func (header *rpmheader) getHeaderRange() (uint64, uint64) {
	return 0, 0
}

