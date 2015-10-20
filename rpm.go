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
import "os"
import "encoding/binary"

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
	header      C.Header
	path        string
	startOffset *uint64
	endOffset   *uint64
}

// openRPM open a RPM file and returns a rpmheader object where you could do various RPM related operations on.
func (ts rpmts) openRPM(path string) (*rpmheader, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	cMode := C.CString("r")
	defer C.free(unsafe.Pointer(cMode))
	header := rpmheader{path: path}

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
	name   string
	value  C.rpmTagVal
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
	if ret == 0 || C.rpmtdCount(&td) != 1 {
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

// getHeaderRange return the byte range of the header in the RPM file as
// (startOffset, endOffset, nil). It returns a non-nil error on errors
func (header *rpmheader) getHeaderRange() (uint64, uint64, error) {
	if header.startOffset != nil && header.endOffset != nil {
		return *header.startOffset, *header.endOffset, nil
	}

	file, err := os.Open(header.path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	if _, err = file.Seek(104, 0); err != nil {
		return 0, 0, err
	}

	var sigIndex uint32
	if err = binary.Read(file, binary.BigEndian, &sigIndex); err != nil {
		return 0, 0, err
	}

	var sigData uint32
	if err = binary.Read(file, binary.BigEndian, &sigData); err != nil {
		return 0, 0, err
	}

	sigIndexSize := sigIndex * 16
	sigSize := sigData + sigIndexSize
	startOffset := uint64(112 + sigSize)
	header.startOffset = &startOffset
	if distToBoundary := sigSize % 8; distToBoundary != 0 {
		*header.startOffset += uint64(8 - distToBoundary)
	}

	if _, err = file.Seek(int64(*header.startOffset)+8, 0); err != nil {
		return 0, 0, err
	}

	var hdrIndex uint32
	if err = binary.Read(file, binary.BigEndian, &hdrIndex); err != nil {
		return 0, 0, err
	}

	var hdrData uint32
	if err = binary.Read(file, binary.BigEndian, &hdrData); err != nil {
		return 0, 0, err
	}
	hdrIndexSize := hdrIndex * 16
	hdrSize := hdrData + hdrIndexSize + 16
	endOffset := *header.startOffset + uint64(hdrSize)
	header.endOffset = &endOffset

	return *header.startOffset, *header.endOffset, nil
}

/* For how to get header start/end
   def _get_header_byte_range(self):
       """takes an rpm file or fileobject and returns byteranges for location of the header"""
       if self._hdrstart and self._hdrend:
           return (self._hdrstart, self._hdrend)


       fo = open(self.localpath, 'r')
       #read in past lead and first 8 bytes of sig header
       fo.seek(104)
       # 104 bytes in
       binindex = fo.read(4)
       # 108 bytes in
       (sigindex, ) = struct.unpack('>I', binindex)
       bindata = fo.read(4)
       # 112 bytes in
       (sigdata, ) = struct.unpack('>I', bindata)
       # each index is 4 32bit segments - so each is 16 bytes
       sigindexsize = sigindex * 16
       sigsize = sigdata + sigindexsize
       # we have to round off to the next 8 byte boundary
       disttoboundary = (sigsize % 8)
       if disttoboundary != 0:
           disttoboundary = 8 - disttoboundary
       # 112 bytes - 96 == lead, 8 = magic and reserved, 8 == sig header data
       hdrstart = 112 + sigsize  + disttoboundary

       fo.seek(hdrstart) # go to the start of the header
       fo.seek(8,1) # read past the magic number and reserved bytes

       binindex = fo.read(4)
       (hdrindex, ) = struct.unpack('>I', binindex)
       bindata = fo.read(4)
       (hdrdata, ) = struct.unpack('>I', bindata)

       # each index is 4 32bit segments - so each is 16 bytes
       hdrindexsize = hdrindex * 16
       # add 16 to the hdrsize to account for the 16 bytes of misc data b/t the
       # end of the sig and the header.
       hdrsize = hdrdata + hdrindexsize + 16

       # header end is hdrstart + hdrsize
       hdrend = hdrstart + hdrsize
       fo.close()
       self._hdrstart = hdrstart
       self._hdrend = hdrend

       return (hdrstart, hdrend)

   hdrend = property(fget=lambda self: self._get_header_byte_range()[1])
   hdrstart = property(fget=lambda self: self._get_header_byte_range()[0])

*/
