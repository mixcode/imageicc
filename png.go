//
// read embedded ICC profile from a PNG file
//
// PNG spec
// https://www.w3.org/TR/2003/REC-PNG-20031110/
//

package imageicc

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"hash"
	"hash/crc32"
	"io"

	bst "github.com/mixcode/binarystruct"
)

var (
	pngHeader = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a} // PNG file header
)

// PNG image is a list of chunks.
type png struct {
	Chunk       []pngChunk       // chunks appear in the PNG
	ChunkByType map[string][]int // [type] -> [ChunkIdx, ChunkIdx, ...]
}

// a PNG chunk. {DataLen, Type, [DATA], CRC32}
// Value is stored in big-endian.
type pngChunk struct {
	DataLen    int    `binary:"uint32"`  // size of actual data
	Type       string `binary:"[4]byte"` // type is 4-byte char sequence
	DataOffset int64  `binary:"ignore"`  // Offset of actual data in the file stream
}

// Parse PNG and get type, offset and size of chunks
func parsePNG(in io.ReadSeeker) (parsedPNG *png, err error) {
	// read PNG header
	h := make([]byte, len(pngHeader))
	sz, err := in.Read(h)
	if err != nil {
		return
	}
	if sz != len(pngHeader) || !bytes.Equal(h, pngHeader) {
		err = fmt.Errorf("invalid PNG header")
		return
	}

	newPNG := png{
		Chunk:       make([]pngChunk, 0),
		ChunkByType: make(map[string][]int),
	}

	var offset int64
	offset, err = in.Seek(0, io.SeekCurrent)
	if err != nil {
		return
	}
	for {
		// read chunk header
		var ch pngChunk
		_, err = bst.Read(in, bst.BigEndian, &ch)
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}
		ch.DataOffset = offset + 8 // 8: chunk header size

		// add the new chunk info
		newPNG.Chunk = append(newPNG.Chunk, ch)
		if newPNG.ChunkByType[ch.Type] == nil {
			newPNG.ChunkByType[ch.Type] = make([]int, 0)
		}
		newPNG.ChunkByType[ch.Type] = append(newPNG.ChunkByType[ch.Type], len(newPNG.Chunk)-1)

		// skip the chunk data and CRC32 value
		offset, err = in.Seek(int64(ch.DataLen+4), io.SeekCurrent) // +4 to skip CRC32 value
		if err != nil {
			return
		}

		if ch.Type == "IEND" { // IEND: Image trailer
			// the end of PNG data stream found
			break
		}
	}

	return &newPNG, nil
}

// crcReader is a reader with a built-in CRC32 calculator
type crcReader struct {
	R      io.Reader
	Crc    hash.Hash32
	ReadSz int
}

// a reader & crc32 calculator
func newCrcReader(r io.Reader) *crcReader {
	return &crcReader{R: r, Crc: crc32.NewIEEE()}
}

// reset CRC calculator
func (c *crcReader) ResetCRC(initialData []byte) {
	c.Crc.Reset()
	c.ReadSz = 0
	if initialData != nil {
		c.Crc.Write(initialData)
	}
}

// read data and update CRC32
func (c *crcReader) Read(p []byte) (n int, err error) {
	n, err = c.R.Read(p)
	if err != nil {
		return
	}
	i, err := c.Crc.Write(p[:n])
	c.ReadSz += i
	return
}

// Read ICC profile embedded in a PNG file.
// profileName is a string included in the PNG along with the ICC profile.
// If there is no ICC profile then nil data and no error is returned.
func LoadICCfromPNG(in io.ReadSeeker) (iccProfile []byte, err error) {
	iccProfile, _, err = LoadICCfromPNGWithName(in)
	return
}

// Read ICC profile and profile name embedded in a PNG file.
// profileName is a string included in the PNG along with the ICC profile.
// If there is no ICC profile then nil data and no error is returned.
func LoadICCfromPNGWithName(in io.ReadSeeker) (iccProfile []byte, profileName string, err error) {
	img, err := parsePNG(in)
	if err != nil {
		return
	}
	l := img.ChunkByType["iCCP"] // ICC profile type chunk
	if len(l) < 1 {
		// PNG does not contain an ICC profile
		// no error; just return nil
		return
	}
	ch := img.Chunk[l[0]] // use the first chunk

	// prepare a CRC32 calculator
	r := newCrcReader(in)
	r.ResetCRC([]byte(ch.Type)) // start a new chunk
	_, err = in.Seek(ch.DataOffset, io.SeekStart)
	if err != nil {
		return
	}

	// read an icc profile chunk
	var iccpChunk struct {
		Name              string `binary:"zstring"` // ICC profile name
		CompressionMethod byte
	}
	sz, err := bst.Read(r, bst.BigEndian, &iccpChunk)
	if err != nil {
		return
	}
	// decompress actual ICC profile chunk
	if iccpChunk.CompressionMethod != 0 {
		err = fmt.Errorf("unknown compression method: %d", iccpChunk.CompressionMethod)
		return
	}
	zl, err := zlib.NewReader(io.LimitReader(r, int64(ch.DataLen-sz)))
	if err != nil {
		return
	}
	iccProfile, err = io.ReadAll(zl)
	zl.Close()
	if err != nil {
		return
	}

	// Check Chunk CRC
	var chunkCRC32 uint32
	_, err = bst.Read(in, bst.BigEndian, &chunkCRC32)
	if err != nil {
		return
	}
	if chunkCRC32 != r.Crc.Sum32() {
		err = fmt.Errorf("chunk %s has invalid CRC", ch.Type)
		return
	}

	return iccProfile, iccpChunk.Name, nil
}
