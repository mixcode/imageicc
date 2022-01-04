package imageicc

import (
	// "fmt"
	// "os"

	"hash/crc32"
	"os"
	"testing"
)

func TestICCfromPNG(t *testing.T) {
	var err error

	pngname := "_testdata/rgb-to-gbr-test.png"
	const iccsum = 0x7f26f21e

	fi, err := os.Open(pngname)
	if err != nil {
		t.Fatal(err)
	}
	defer fi.Close()

	icc, name, err := LoadICCfromPNGWithName(fi)
	if err != nil {
		t.Fatal(err)
	}
	_ = name

	sum := crc32.ChecksumIEEE(icc)
	if sum != iccsum {
		t.Fatalf("checksum does not match")
	}
	//fmt.Printf("name, sz: %s, %d, %08x\n", name, len(icc), sum)

	// test: write to a file
	//err = os.WriteFile("_"+name+".icc", icc, 0644)
	//if err != nil {
	//	t.Fatal(err)
	//}
}

func TestICCfromJPG(t *testing.T) {
	var err error

	jpgname := "_testdata/rgb-to-gbr-test copy.jpg"
	const iccsum = 0x7f26f21e

	fi, err := os.Open(jpgname)
	if err != nil {
		t.Fatal(err)
	}
	defer fi.Close()

	icc, err := LoadICCfromJPG(fi)
	if err != nil {
		t.Fatal(err)
	}

	sum := crc32.ChecksumIEEE(icc)
	if sum != iccsum {
		t.Fatalf("checksum does not match")
	}
	//fmt.Printf("icc profile sz: %d, sum: %08x\n", len(icc), sum)
}

func TestICCfromGIF(t *testing.T) {
	var err error

	gifname := "_testdata/icc-color-profile.gif"
	const iccsum = 0x28f60bbf

	fi, err := os.Open(gifname)
	if err != nil {
		t.Fatal(err)
	}
	defer fi.Close()

	icc, err := LoadICCfromGIF(fi)
	if err != nil {
		t.Fatal(err)
	}

	sum := crc32.ChecksumIEEE(icc)
	//log.Printf("icc profile sz: %d, sum: %08x\n", len(icc), sum)
	if sum != iccsum {
		t.Fatalf("checksum does not match")
	}

	/*
		// test: write to a file
		err = os.WriteFile("_test_gif.icc", icc, 0644)
		if err != nil {
			t.Fatal(err)
		}
	*/
}

func TestICCfromTIFF(t *testing.T) {
	var err error

	tifname := "_testdata/rgb-to-gbr-test copy.tif"
	const iccsum = 0x7f26f21e

	fi, err := os.Open(tifname)
	if err != nil {
		t.Fatal(err)
	}
	defer fi.Close()

	icc, err := LoadICCfromTIFF(fi)
	if err != nil {
		t.Fatal(err)
	}

	sum := crc32.ChecksumIEEE(icc)
	if sum != iccsum {
		t.Fatalf("checksum does not match")
	}
	//fmt.Printf("icc profile sz: %d, sum: %08x\n", len(icc), sum)

	/*
		// test: write to a file
		err = os.WriteFile("_test_gif.icc", icc, 0644)
		if err != nil {
			t.Fatal(err)
		}
	*/
}
