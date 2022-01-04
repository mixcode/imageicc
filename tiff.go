//
// read embedded ICC profile from a TIFF file
//
// TIFF spec
// https://www.adobe.io/open/standards/TIFF.html
//

package imageicc

import (
	"fmt"
	"io"

	bst "github.com/mixcode/binarystruct"
)

// TIF Image File Directory
type tifIFD struct {
	NumEntry      int           `binary:"uint16"`     // number of entries in the IFD
	DirEntry      []tifDirEntry `binary:"[NumEntry]"` // entries
	OffsetNextIFD int64         `binary:"uint32"`     // file offset to the next IFD
}

type tifDirEntry struct {
	Tag   uint16 // Tag id of the entry
	Type  uint16 // type code of the value
	Count int    `binary:"uint32"` // number of values
	Value uint32 `binary:"uint32"` // value if it fits in 4-bytes, or offset to the value
}

// name of TIF value type
const (
	tifTypeBYTE      = 1  // byte/uint8
	tifTypeASCII     = 2  // []byte (with a trailing zero)
	tifTypeSHORT     = 3  // uint16
	tifTypeLONG      = 4  // uint32
	tifTypeRATIONAL  = 5  // unsigned rational number, fraction of two uint32s (uint32[1] / uint32[0])
	tifTypeSBYTE     = 6  // signed byte
	tifTypeUNDEFINED = 7  // byte
	tifTypeSSHORT    = 8  // int16
	tifTypeSLONG     = 9  // int32
	tifTypeSRATIONAL = 10 // signed rational number, fraction of two uint32s (uint32[1] / uint32[0])
	tifTypeFLOAT     = 11 // float32
	tifTypeDOUBLE    = 12 // float64
)

var (
	// byte size of TIF value type
	tifTypeSize = []int{
		0,
		1, 1, 2, 4, 8, // BYTE, ASCII, SHORT, LONG, RATIONAL
		1, 1, 2, 4, 8, // SBYTE, UNDEFINED, SSHORT, SLONG, SRATIONAL
		4, 8, // FLOAT, DOUBLE
	}
)

func (d *tifDirEntry) fetchRawData(in io.ReadSeeker, endian bst.ByteOrder) (b []byte, err error) {
	sz := d.Count * tifTypeSize[d.Type]
	if sz < 4 { // data fit in the d.Value field
		// revert value to []byte
		b, err = bst.Marshal(d.Value, endian)
		if err == nil && b != nil {
			b = b[:sz]
		}
		return
	}
	// d.Value is the file offset to the data
	_, err = in.Seek(int64(d.Value), io.SeekStart)
	if err != nil {
		return
	}
	b = make([]byte, sz)
	_, err = io.ReadFull(in, b)
	if err != nil {
		b = nil
	}
	return
}

func (d *tifDirEntry) getBytes(in io.ReadSeeker, endian bst.ByteOrder) (b []byte, err error) {
	if d.Type != tifTypeBYTE && d.Type != tifTypeSBYTE && d.Type != tifTypeASCII && d.Type != tifTypeUNDEFINED {
		err = fmt.Errorf("not a byte type")
		return
	}
	return d.fetchRawData(in, endian)
}

func (d *tifDirEntry) getString(in io.ReadSeeker, endian bst.ByteOrder) (s string, err error) {
	if d.Type != tifTypeASCII {
		err = fmt.Errorf("not a ASCII type")
		return
	}
	buf, err := d.fetchRawData(in, endian)
	if err != nil {
		return
	}
	//for len(buf) > 1 && buf[len(buf)-1] == 0 { // remove trailing zeros
	if len(buf) > 1 { // remove the trailing zero
		buf = buf[:len(buf)-1]
	}
	return string(buf), nil
}

// assume the number is single int value, then get the value
func (d *tifDirEntry) getInt() (n int64, err error) {
	if d.Count != 1 {
		err = fmt.Errorf("must be a single value")
		return
	}
	switch d.Type {
	case tifTypeBYTE, tifTypeSHORT, tifTypeLONG, tifTypeUNDEFINED:
		n = int64(d.Value)
	case tifTypeSBYTE:
		n = int64(int8(byte(d.Value)))
	case tifTypeSSHORT:
		n = int64(int16(uint16(d.Value)))
	case tifTypeSLONG:
		n = int64(int32(uint32(d.Value)))
	default:
		err = fmt.Errorf("not an integer type")
	}
	return
}

// get integer values
func (d *tifDirEntry) getIntArray(in io.ReadSeeker, endian bst.ByteOrder) (n []int64, err error) {
	switch d.Type {
	case tifTypeBYTE, tifTypeSHORT, tifTypeLONG, tifTypeUNDEFINED,
		tifTypeSBYTE, tifTypeSSHORT, tifTypeSLONG:
		// do nothing
	default:
		err = fmt.Errorf("not an integer type")
		return
	}

	buf, err := d.fetchRawData(in, endian)
	if err != nil {
		return
	}

	switch d.Type {
	case tifTypeBYTE, tifTypeUNDEFINED:
		var l struct {
			N []int64 `binary:"[]byte"`
		}
		_, err = bst.Unmarshal(buf, endian, &l)
		if err != nil {
			return
		}
		n = l.N
	case tifTypeSHORT:
		var l struct {
			N []int64 `binary:"[]uint16"`
		}
		_, err = bst.Unmarshal(buf, endian, &l)
		if err != nil {
			return
		}
		n = l.N
	case tifTypeLONG:
		var l struct {
			N []int64 `binary:"[]uint32"`
		}
		_, err = bst.Unmarshal(buf, endian, &l)
		if err != nil {
			return
		}
		n = l.N
	case tifTypeSBYTE:
		var l struct {
			N []int64 `binary:"[]int8"`
		}
		_, err = bst.Unmarshal(buf, endian, &l)
		if err != nil {
			return
		}
		n = l.N
	case tifTypeSSHORT:
		var l struct {
			N []int64 `binary:"[]int16"`
		}
		_, err = bst.Unmarshal(buf, endian, &l)
		if err != nil {
			return
		}
		n = l.N
	case tifTypeSLONG:
		var l struct {
			N []int64 `binary:"[]int32"`
		}
		_, err = bst.Unmarshal(buf, endian, &l)
		if err != nil {
			return
		}
		n = l.N
	default:
		err = fmt.Errorf("not an integer value")
	}
	return
}

// Parse TIFF tags and find an embedded ICC profile
func LoadICCfromTIFF(in io.ReadSeeker) (iccProfile []byte, err error) {

	buf := make([]byte, 16)

	// read TIFF header
	_, err = io.ReadFull(in, buf[:8])
	if err != nil {
		return
	}
	var endian bst.ByteOrder
	// First two bytes indicate the byte order
	if buf[0] == 'I' && buf[1] == 'I' {
		// "II\0x2a\0" : Little-endian TIFF
		endian = bst.LittleEndian
	} else if buf[0] == 'M' && buf[1] == 'M' {
		// "MM\0\0x2a" : big-endian TIFF
		endian = bst.BigEndian
	} else {
		err = fmt.Errorf("invalid TIF header")
		return
	}
	var tifHeader struct {
		Magic     uint16 // a magic number, 42
		OffsetIfd int64  `binary:"uint32"` // offset to the first Image File Directory
	}
	_, err = bst.Unmarshal(buf[2:8], endian, &tifHeader)
	if err != nil {
		return
	}
	if tifHeader.Magic != 42 {
		// 42: the Answer to the Ultimate Question of Life, the Universe, and Everything.
		err = fmt.Errorf("invalid TIF header")
		return
	}

	// read image file directories
	ifd := tifIFD{DirEntry: make([]tifDirEntry, 0)}
	ifdOffset := tifHeader.OffsetIfd
	for ifdOffset != 0 {
		// seek to the ifd offset
		_, err = in.Seek(ifdOffset, io.SeekStart)
		if err != nil {
			return
		}
		// read single ifd block
		var lfd tifIFD
		_, err = bst.Read(in, endian, &lfd)
		if err != nil {
			return
		}
		// append new ifd to the main ifd table
		ifd.NumEntry += lfd.NumEntry
		ifd.DirEntry = append(ifd.DirEntry, lfd.DirEntry...)
		ifdOffset = ifd.OffsetNextIFD
	}

	// Seek for a ICC profile tag
	for _, d := range ifd.DirEntry {
		/*
			//!! test: print string values
			if d.Type == tifTypeASCII {
				s, e := d.getString(in, endian)
				if e != nil {
					return nil, e
				}
				log.Printf("Tag %04x: [%s]", d.Tag, s)
			}
		*/

		switch d.Tag {

		case 0x8773: // 0x8773: TIFFTAG_ICCPROFILE
			// ICC profile found; load the data block
			buf, err = d.getBytes(in, endian)
			if err != nil {
				return
			}
			return buf, nil

			// case 0x8769: // 0x8769: TIFTAG_EXIFIFD
			// case 0x8825: // 0x8825: TIFTAG_GPSIFD
			// case 0x9000: // 0x9000: ExifVersion

		}

	}

	return
}
