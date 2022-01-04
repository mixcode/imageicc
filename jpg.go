//
// read embedded ICC profile from a jpg file
//
// jpeg/JFIF format spec
// https://www.iso.org/standard/54989.html
//

package imageicc

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
)

const (
	// jpeg markers
	markerNONE = 0x00 // 0xff 0x00 is not a marker, just an offsetted 0xff byte
	markerSOF0 = 0xc0 // Start Of Frame (Baseline Sequential).
	markerSOF1 = 0xc1 // Start Of Frame (Extended Sequential).
	markerSOF2 = 0xc2 // Start Of Frame (Progressive).
	markerDHT  = 0xc4 // Define Huffman Table.
	markerRST0 = 0xd0 // ReSTart (0).
	markerRST7 = 0xd7 // ReSTart (7).
	markerSOI  = 0xd8 // Start Of Image.
	markerEOI  = 0xd9 // End Of Image.
	markerSOS  = 0xda // Start Of Scan.
	markerDQT  = 0xdb // Define Quantization Table.
	markerDRI  = 0xdd // Define Restart Interval.
	markerCOM  = 0xfe // COMment.
	// "APPlication specific" markers aren't part of the JPEG spec per se,
	// but in practice, their use is described at
	// https://www.sno.phy.queensu.ca/~phil/exiftool/TagNames/JPEG.html
	markerAPP0  = 0xe0
	markerAPP2  = 0xe2
	markerAPP14 = 0xee
	markerAPP15 = 0xef
)

// Read ICC profile embedded in a JPG file.
// If there is no ICC profile then nil data and no error is returned.
func LoadICCfromJPG(in io.ReadSeeker) (iccProfile []byte, err error) {

	buf := make([]byte, 16)

	// Read jpg SOI
	_, err = io.ReadFull(in, buf[:2])
	if err != nil {
		return
	}
	if buf[0] != 0xff || buf[1] != markerSOI { // 0xff 0xd8, Start of Image marker
		err = fmt.Errorf("start-of-image marker not found")
		return
	}

	var iccData *bytes.Buffer
	var iccLastIndex, iccIndexMax int

	// Read segments
	numComponents := 0
	for {
		// read segment marker
		_, err = io.ReadFull(in, buf[:2])
		if err != nil {
			return
		}
		if buf[0] != 0xff {
			log.Printf("JFIF warning: unaligned segment header") //!!
		}
		for buf[0] != 0xff {
			buf[0] = buf[1]
			_, err = io.ReadFull(in, buf[1:2])
			if err != nil {
				return
			}
		}
		marker := buf[1]
		if marker == markerEOI { // End-of-Image
			break
		}
		if markerRST0 <= marker && marker <= markerRST7 { // Restart markers
			// ignore the RST marker
			continue
		}
		// read segment length
		_, err = io.ReadFull(in, buf[:2])
		if err != nil {
			return
		}
		segLen := int(buf[0])<<8 + int(buf[1]) - 2 // segment length includes the length itself

		switch marker {
		case markerAPP0: // possible JFIF header
			if segLen > 5 {
				_, err = io.ReadFull(in, buf[:5])
				if err != nil {
					return
				}
				jfif := (buf[0] == 'J' && buf[1] == 'F' && buf[2] == 'I' && buf[3] == 'F' && buf[4] == 0)
				if !jfif {
					err = fmt.Errorf("APP0 segment has unknown signature")
					return
				}
				segLen -= 5
				if segLen > 0 {
					in.Seek(int64(segLen), io.SeekCurrent)
				}
			}
		case markerAPP2: // APP2 markers may contain a ICC profile
			if segLen >= 0x0e { // {"ICC_PROFILE\0", chunknum, chunkmax}
				// read ICC_PROFILE chunk header
				_, err = io.ReadFull(in, buf[:0x0e])
				if err != nil {
					return
				}
				segLen -= 0x0e
				if string(buf[:0x0b]) == "ICC_PROFILE" {
					// max size of a segment is around 64KBytes, so large data is divided into multiple segs
					idx, count := int(buf[0x0c]), int(buf[0x0d])
					if iccIndexMax == 0 {
						iccIndexMax = count
					}
					if iccIndexMax != count {
						err = fmt.Errorf("icc profile segment count mismatch")
						return
					}
					if idx != iccLastIndex+1 {
						err = fmt.Errorf("icc profile segments are not linearly stored")
						return
					}

					// load a chunk of ICC profile data
					if iccData == nil {
						iccData = new(bytes.Buffer)
					}
					_, err = io.CopyN(iccData, in, int64(segLen))
					if err != nil {
						return
					}
					iccLastIndex++

					if iccLastIndex == iccIndexMax {
						// ICC profile succesfully loaded. return early.
						b := iccData.Bytes()
						if len(b) == 0 {
							b = nil
						}
						return b, nil
					}
				}
			}
			if segLen > 0 {
				in.Seek(int64(segLen), io.SeekCurrent)
			}

		case markerSOF0, markerSOF1, markerSOF2: // Start of Frame
			// SOF0: baseline, SOF2: progressive
			if numComponents != 0 {
				// SOF markers already found
				err = fmt.Errorf("multiple SOF markers")
				return
			}
			switch segLen {
			case 6 + 3*1: // 1 plane: Grayscale
				numComponents = 1
			case 6 + 3*3: // 3 planes: YCbCr or RGB
				numComponents = 3
			case 6 + 3*4: // 4 planes: YCbCrK or RGBK
				numComponents = 4
			}
			// skip actual data
			in.Seek(int64(segLen), io.SeekCurrent)

		case markerSOS: // start-of-scan
			_, err = skipSOS(in, numComponents, segLen)
			if err != nil {
				return
			}

		default:
			// skip the entire segment
			in.Seek(int64(segLen), io.SeekCurrent)
		}
	}

	return
}

// skip a Start of Scan segment
func skipSOS(in io.ReadSeeker, numComponents int, headerLen int) (n int, err error) {

	if numComponents == 0 {
		// SOF markers not found
		err = fmt.Errorf("no SOF marker")
		return
	}
	if headerLen < 6 || 4+2*numComponents < headerLen || headerLen%2 != 0 {
		err = fmt.Errorf("SOS has wrong length")
		return
	}

	// save current position
	offset, err := in.Seek(0, io.SeekCurrent)
	if err != nil {
		return
	}
	readSz := 0

	// read start-of-scan segment header
	buf := make([]byte, headerLen)
	i, err := io.ReadFull(in, buf)
	if err != nil {
		return
	}
	readSz += i

	// Scan for next chunk header, 0xff 0xXX
	br := bufio.NewReader(in)
	var prevByte, c byte
	for {
		prevByte = c
		c, err = br.ReadByte()
		if err != nil {
			return
		}
		readSz++
		// in entropy encoding, 0xff 00 is a single-byte 0xff
		// 0xff followed by a non-zero byte is a segment header
		if prevByte == 0xff && c != 0 {
			break
		}
	}

	readSz -= 2
	_, err = in.Seek(offset+int64(readSz), io.SeekStart)
	if err != nil {
		return
	}
	return readSz, nil
}
