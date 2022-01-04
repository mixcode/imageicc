//
// read embedded ICC profile from a gif file
//
// GIF spec
// https://www.w3.org/Graphics/GIF/spec-gif89a.txt
//

package imageicc

import (
	"bytes"
	"fmt"
	"io"

	bst "github.com/mixcode/binarystruct"
)

const (
	// extension block codes
	gifextGraphicControl = 0xf9 // 0x21 0xf9: Graphic Control Extension block
	gifextComment        = 0xfe // 0x21 0xfe: Comment extension block
	gifextPlainText      = 0x01 // 0x21 0x01: Plan Text block
	gifextApplication    = 0xff // 0x21 0xff: Application extension
)

// Read ICC profile embedded in a PNG file.
// If there is no ICC profile then nil data and no error is returned.
func LoadICCfromGIF(in io.ReadSeeker) (iccProfile []byte, err error) {

	buf := make([]byte, 1024)
	getC := func(r io.Reader) (byte, error) { // read a char
		_, e := io.ReadFull(r, buf[:1])
		if e != nil {
			return 0, e
		}
		return buf[0], nil
	}
	// read a sub block
	readBlock := func(r io.Reader) ([]byte, error) {
		sz, err := getC(r)
		if err != nil {
			return nil, err
		}
		if sz == 0 {
			return nil, nil
		}
		b := make([]byte, sz)
		_, err = io.ReadFull(r, b)
		if err != nil {
			return nil, err
		}
		return b, nil
	}
	// read all sub-blocks
	readBlocks := func(r io.Reader) ([]byte, error) {
		var buf bytes.Buffer
		for {
			sz, err := getC(r)
			if err != nil {
				return nil, err
			}
			if sz == 0 {
				break
			}
			_, err = io.CopyN(&buf, r, int64(sz))
			if err != nil {
				return nil, err
			}
		}
		b := buf.Bytes()
		if len(b) == 0 {
			b = nil
		}
		return b, nil
	}

	// skip subblocks
	skipBlocks := func(r io.ReadSeeker) (n int, err error) {
		// read multiple 256-byte blocks
		for {
			var sz byte
			sz, err = getC(r)
			if err != nil {
				return
			}
			if sz == 0 { // end of sub blocks
				return
			}
			var i int64
			//i, err = io.ReadFull(r, buf[:sz])
			i, err = r.Seek(int64(sz), io.SeekCurrent)
			if err != nil {
				return
			}
			n += int(i)
		}
	}

	// read GIF header
	var gifHeader struct {
		Version          string `binary:"[6]byte"`
		Width, Height    int    `binary:"uint16"`
		Flag             byte
		BGColorIndex     byte
		PixelAspectRatio byte
	}
	_, err = bst.Read(in, bst.LittleEndian, &gifHeader)
	if err != nil {
		return
	}
	if gifHeader.Version[:3] != "GIF" ||
		gifHeader.Version[3] < '0' || gifHeader.Version[3] > '9' ||
		gifHeader.Version[4] < '0' || gifHeader.Version[4] > '9' ||
		gifHeader.Version[5] < 'a' || gifHeader.Version[5] > 'z' {
		err = fmt.Errorf("invalid GIF header")
		return
	}

	// process the global color table
	flagGlobalColorTable := (gifHeader.Flag & 0x80) != 0
	szGlobalColorTable := 0
	if flagGlobalColorTable {
		szGlobalColorTable = 1 << (1 + gifHeader.Flag&0x7)
		// global color table is a color palette of RGB triplets
		_, err = in.Seek(int64(szGlobalColorTable*3), io.SeekCurrent)
		if err != nil {
			return
		}
	}

	// read blocks
	for {
		var c byte
		c, err = getC(in)
		if err != nil {
			return
		}
		if c == 0x3b { // 0x3b: GIF trailer
			// the end-of-image
			break
		}

		switch c {
		case 0x2c: // 0x2c: Image descriptor

			// load image description
			var imgDesc struct {
				Left, Top, Width, Height int `binary:"uint16"`
				Flag                     byte
			}
			_, err = bst.Read(in, bst.LittleEndian, &imgDesc)
			if err != nil {
				return
			}
			// process the local color table
			flagLocalColorTable := (imgDesc.Flag & 0x80) != 0
			szLocalColorTable := 0
			if flagLocalColorTable {
				szLocalColorTable = 1 << (1 + imgDesc.Flag&0x7)
				// global color table is a color palette of RGB triplets
				_, err = in.Seek(int64(szLocalColorTable*3), io.SeekCurrent)
				if err != nil {
					return
				}
			}

			// read the LZW minimum code size
			_, err = getC(in)
			if err != nil {
				return
			}
			// lzwMinimumCodeSize := c

			// skip LZW-compressed image data blocks
			_, err = skipBlocks(in)
			if err != nil {
				return
			}

		case 0x21: // 0x21: extension block
			c, err = getC(in) // extension block type code
			if err != nil {
				return
			}
			switch c {
			//case gifextGraphicControl: // 0x21 0xf9: Graphic Control Extension block
			//case gifextComment:	// 0x21 0xfe: Comment extension block
			//case gifextPlainText: // 0x21 0x01: Plan Text block
			case gifextApplication: // 0x21 0xff: Application extension
				var block []byte
				block, err = readBlock(in)
				if err != nil {
					return
				}
				if len(block) != 8+3 { // ID + Auth
					err = fmt.Errorf("application extension block header size mismatch")
					return
				}
				appId := string(block[:8])   // application identifier string. 8 chars.
				appAuth := string(block[8:]) // application auth code. 3 chars.

				if appId == "ICCRGBG1" && appAuth == "012" { // "ICCRGBG1": ICC profile application extension
					// Load embedded ICC profile
					block, err = readBlocks(in)
					if err != nil {
						return
					}
					// ICC profile OK
					return block, nil
				}

				// unknown application ID
				_, err = skipBlocks(in)
				if err != nil {
					return
				}

			default:
				_, err = skipBlocks(in)
				if err != nil {
					return
				}
			}

		default:
			err = fmt.Errorf("unknown chunk type %x", c)
			return
		}
	}

	// end of file
	return
}
