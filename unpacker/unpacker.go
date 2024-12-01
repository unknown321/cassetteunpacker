package unpacker

import (
	"archive/zip"
	"bytes"
	"casetteunpacker/metadata"
	"casetteunpacker/tapeconfig"
	"compress/bzip2"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xi2/xz"
	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
	"google.golang.org/protobuf/proto"
)

type Unpacker struct {
	Key []byte
	IV  []byte
}

func FromKey(key string) *Unpacker {
	u := &Unpacker{}
	u.Key = []byte(key[0:32])
	u.IV = []byte(key[32:])
	return u
}

func Decrypt(key, iv []byte, filename string) []byte {
	slog.Info("decrypting firmware")
	enc, _ := os.ReadFile(filename)

	//res, _ := os.OpenFile("res.zip", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	//w := bufio.NewWriter(res)

	block, err := aes.NewCipher(key)
	if err != nil {
		fmt.Println("key error", err)
	}

	ecb := cipher.NewCBCDecrypter(block, iv)

	crypt := enc[128:]

	decrypted := make([]byte, len(crypt))
	ecb.CryptBlocks(decrypted, crypt)
	trimmed := PKCS5Trimming(decrypted)
	return trimmed
}

func GetPayload(data []byte) []byte {
	slog.Info("getting payload")
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		panic(err.Error())
	}

	f, err := zr.Open("payload.bin")
	if err != nil {
		panic(err.Error())
	}

	var bz []byte
	if bz, err = io.ReadAll(f); err != nil {
		panic(err.Error())
	}

	return bz
}

func GetMetadata(data []byte) (*metadata.DeltaArchiveManifest, uint64) {
	slog.Info("getting metadata")
	var err error
	br := bytes.NewReader(data)

	var headerSize uint64 = 0

	magic := make([]byte, 4)
	if _, err = br.Read(magic); err != nil {
		panic(err.Error())
	}
	if !bytes.Equal(magic, []byte("CrAU")) {
		panic("bad magic")
	}

	headerSize += 4

	var formatVer uint64
	if err = binary.Read(br, binary.BigEndian, &formatVer); err != nil {
		panic(err.Error())
	}
	headerSize += 8

	var manifestSize uint64
	if err = binary.Read(br, binary.BigEndian, &manifestSize); err != nil {
		panic(err.Error())
	}
	headerSize += 8

	var metadataSigSize uint32
	if formatVer > 1 {
		if err = binary.Read(br, binary.BigEndian, &metadataSigSize); err != nil {
			panic(err.Error())
		}
		headerSize += 4
	}

	damB := make([]byte, manifestSize)
	if _, err = io.ReadFull(br, damB); err != nil {
		panic(err.Error())
	}

	headerSize += manifestSize + uint64(metadataSigSize)

	dam := metadata.DeltaArchiveManifest{}

	if err = proto.Unmarshal(damB, &dam); err != nil {
		panic(err.Error())
	}

	return &dam, headerSize
}

func DumpPartition(payload []byte, name string, manifest *metadata.DeltaArchiveManifest, outname string, baseOffset uint64) {
	var part []*metadata.InstallOperation
	for _, p := range manifest.GetPartitions() {
		if p.GetPartitionName() == name {
			part = p.GetOperations()
			break
		}
	}

	if len(part) < 1 {
		panic("no system part found")
	}

	slog.Info("dumping partition", "partition name", name)

	var out *os.File
	var err error

	if out, err = os.OpenFile(outname, os.O_TRUNC|os.O_RDWR|os.O_CREATE, 0644); err != nil {
		panic(err.Error())
	}

	br := bytes.NewReader(payload)

	for _, op := range part {
		if _, err = br.Seek(int64(op.GetDataOffset()+baseOffset), 0); err != nil {
			panic(err)
		}

		data := make([]byte, op.GetDataLength())
		if _, err = br.Read(data); err != nil {
			panic(err)
		}

		switch op.GetType() {
		case metadata.InstallOperation_REPLACE:
			if _, err = out.Seek(int64(op.GetDstExtents()[0].GetStartBlock()*uint64(manifest.GetBlockSize())), 0); err != nil {
				panic(err.Error())
			}

			if _, err = out.Write(data); err != nil {
				panic(err)
			}

		case metadata.InstallOperation_REPLACE_XZ:
			xzr, err2 := xz.NewReader(bytes.NewReader(data), 0)
			if err2 != nil {
				panic(err2)
			}

			if _, err = out.Seek(int64(op.GetDstExtents()[0].GetStartBlock()*uint64(manifest.GetBlockSize())), 0); err != nil {
				panic(err.Error())
			}

			if _, err = io.Copy(out, xzr); err != nil {
				panic(err)
			}

		case metadata.InstallOperation_REPLACE_BZ:
			bzr := bzip2.NewReader(bytes.NewReader(data))
			if _, err = out.Seek(int64(op.GetDstExtents()[0].GetStartBlock()*uint64(manifest.GetBlockSize())), 0); err != nil {
				panic(err.Error())
			}
			if _, err = io.Copy(out, bzr); err != nil {
				panic(err)
			}
		default:
			fmt.Printf("unexpected operation %d\n", op.GetType())
		}
	}

	_ = out.Close()

	slog.Info("dumped partition", "partition name", name)
}

func (u *Unpacker) Unpack(filename string) error {
	decrypted := Decrypt(u.Key, u.IV, filename)
	payload := GetPayload(decrypted)
	manifest, headerSize := GetMetadata(payload)
	DumpPartition(payload, "system", manifest, "system.ext2", headerSize)

	var err error
	if err = GetApk(); err != nil {
		return err
	}

	var fileList []string
	if fileList, err = ExtractImages(); err != nil {
		return fmt.Errorf("cannot extract images: %w", err)
	}

	for _, fileName := range fileList {
		if err = PrepareCassette(fileName); err != nil {
			return fmt.Errorf("cannot prepare file %s: %w", fileName, err)
		}
	}

	if err = FilesToDirs(); err != nil {
		return fmt.Errorf("cannot move images to dirs: %w", err)
	}

	return nil
}

func GetApk() error {
	slog.Info("getting apk")
	cmd := exec.Command("e2cp", "system.ext2:/system/priv-app/HighResMediaPlayerApp/HighResMediaPlayerApp.apk", ".")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cannot extract apk: %w", err)
	}
	return nil
}

func ExtractImages() ([]string, error) {
	slog.Info("extracting images")
	prefixLen := len("res/drawable/")

	data, err := os.ReadFile("HighResMediaPlayerApp.apk")
	if err != nil {
		return nil, fmt.Errorf("cannot open apk: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("cannot create zip reader: %w", err)
	}

	_ = os.Mkdir("res", 0755)

	result := []string{}

	for _, zf := range zr.File {
		match, err := filepath.Match("res/drawable/ic_audio_play_*.jpg", zf.Name)
		if err != nil {
			return nil, fmt.Errorf("cannot match files in zip reader: %w", err)
		}

		if !match {
			continue
		}

		newName := path.Join("res", zf.Name[prefixLen:])
		zfr, err := zf.Open()
		if err != nil {
			return nil, fmt.Errorf("cannot open zip file reader: %w", err)
		}

		var f *os.File
		f, err = os.OpenFile(newName, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
		if err != nil {
			return nil, fmt.Errorf("cannot create image file for writing: %w", err)
		}

		if _, err = io.Copy(f, zfr); err != nil {
			return nil, fmt.Errorf("cannot copy data from zip file reader to file on disc: %w", err)
		}
		result = append(result, newName)
	}

	return result, nil
}

var transformMatrix = func(scale, tx, ty float64) f64.Aff3 {
	const cos90, sin90 = 0, 1
	return f64.Aff3{
		scale * cos90, -scale * sin90, tx,
		scale * sin90, +scale * cos90, ty,
	}
}

func PrepareCassette(filename string) error {
	f, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	br := bytes.NewReader(f)

	img, err := jpeg.Decode(br)
	if err != nil {
		return fmt.Errorf("cannot decode jpeg: %w", err)
	}

	var cropR image.Rectangle
	var downscaleR image.Rectangle
	var rotatedR image.Rectangle
	var dstR image.Rectangle

	if strings.Contains(filename, "ic_audio_play_cassette") {
		dstR.Max = image.Point{X: 720, Y: 1200}
		cropR = image.Rectangle{Min: image.Point{X: 0, Y: -80}, Max: image.Point{X: 720, Y: 1200}}
		downscaleR.Max = image.Point{X: 480, Y: 800}
		rotatedR.Max = image.Point{X: 800, Y: 480}
	} else {
		dstR = img.Bounds()
		cropR = img.Bounds()
		downscaleR.Max = image.Point{X: 116, Y: 528}
		rotatedR.Max = image.Point{X: 528, Y: 116}
	}

	dst := image.NewRGBA(dstR)
	draw.Draw(dst, cropR, img, image.Point{}, draw.Src)

	dstDownscaled := image.NewRGBA(downscaleR)
	draw.BiLinear.Scale(dstDownscaled, dstDownscaled.Bounds(), dst, dst.Bounds(), draw.Over, nil)

	dstRotated := image.NewRGBA(rotatedR)
	draw.BiLinear.Transform(dstRotated, transformMatrix(1, float64(dstRotated.Bounds().Max.X), 0), dstDownscaled, dstDownscaled.Bounds(), draw.Over, nil)

	out, err := os.OpenFile(filename, os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("cannot open file after preparing: %w", err)
	}

	if err = jpeg.Encode(out, dstRotated, &jpeg.Options{Quality: 90}); err != nil {
		return fmt.Errorf("cannot encode jpeg: %w", err)
	}

	return nil
}

// FilesToDirs puts cassette files into separate directories
func FilesToDirs() error {
	slog.Info("putting files into ./res")
	var err error

	names := []string{"chf", "metal_master", "other"}
	for _, name := range names {
		n := path.Join("res/reel", name)
		if err = os.MkdirAll(n, 0755); err != nil {
			return fmt.Errorf("cannot create output dir: %w", err)
		}
	}

	var matchesReel []string
	if matchesReel, err = filepath.Glob("res/ic_audio_play_tape_reel_*.jpg"); err != nil {
		return fmt.Errorf("cannot glob: %w", err)
	}

	for _, m := range matchesReel {
		for _, name := range names {
			if !strings.Contains(m, name) {
				continue
			}

			newName := path.Join("res/reel", name, path.Base(m))
			if err = os.Rename(m, newName); err != nil {
				return fmt.Errorf("cannot move file %s -> %s: %w", m, newName, err)
			}
		}

	}

	if err = os.MkdirAll("res/tape", 0755); err != nil {
		return fmt.Errorf("cannot create tape dir: %w", err)
	}

	var matchesTape []string
	if matchesTape, err = filepath.Glob("res/ic_audio_play_cassette_*.jpg"); err != nil {
		return fmt.Errorf("cannot glob: %w", err)
	}

	// extract base cassette name from filename
	r, err := regexp.Compile("ic_audio_play_cassette_(.*)_picture.jpg")
	for _, m := range matchesTape {
		cNames := r.FindSubmatch([]byte(path.Base(m)))
		if len(cNames) < 2 {
			return fmt.Errorf("unexpected file name %s", m)
		}

		tapeName := cNames[1]

		if err = os.MkdirAll(path.Join("res/tape", string(tapeName)), 0755); err != nil {
			return fmt.Errorf("cannot create tape dir %s: %w", string(tapeName), err)
		}

		if err = os.Rename(m, path.Join("res/tape", string(tapeName), path.Base(m))); err != nil {
			return fmt.Errorf("cannot move file %s to tape dir %s: %w", m, string(tapeName), err)
		}

		c := tapeconfig.TapeConfig{
			Reel:       "other",
			ArtistX:    83,
			ArtistY:    77,
			TitleX:     83,
			TitleY:     112,
			ReelX:      134,
			ReelY:      160,
			TitleWidth: 600,
			TextColor:  "#000000",
		}

		switch string(tapeName) {
		case "chf":
			c.Reel = "chf"
		case "metal_master":
			c.Reel = "metal_master"
			c.TitleX = 72
			c.TitleY = 73
			c.ArtistX = -1
			c.ArtistY = -1
			c.TitleWidth = 480 - 72
			c.ReelX = 134.6
			c.ReelY = 160
		case "ucx", "ucx_s":
			c.TitleX = 87
			c.TitleY = 78
			c.ArtistX = 87
			c.ArtistY = 110
		case "metal":
			c.TitleX = 121
			c.TitleY = 45
			c.ArtistX = 121
			c.ArtistY = 81
		}

		if err = c.Write(path.Join("res/tape", string(tapeName))); err != nil {
			return err
		}
	}

	return nil
}

func PKCS5Trimming(encrypt []byte) []byte {
	padding := encrypt[len(encrypt)-1]
	return encrypt[:len(encrypt)-int(padding)]
}
