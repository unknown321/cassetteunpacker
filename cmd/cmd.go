package cmd

import (
	"casetteunpacker/unpacker"
	"flag"
	"log/slog"
	"os"
)

func Run() {
	var key string
	var filename string
	var clean bool
	flag.StringVar(&key, "k", "o0W0mihblfJOPtNQPN8Pc2hLKiTROL5MVERN9OmmkkMNZO2P", "firmware decryption key")
	flag.StringVar(&filename, "f", "NW-A100_0003_V4_04_00_NW_WM_FW.UPG", "path to firmware file")
	flag.BoolVar(&clean, "c", true, "remove intermediate files")
	flag.Parse()

	u := unpacker.FromKey(key)
	if err := u.Unpack(filename); err != nil {
		slog.Error("", "error", err.Error())
		os.Exit(1)
	}

	if clean {
		slog.Info("cleaning up")
		_ = os.Remove("system.ext2")
		_ = os.Remove("HighResMediaPlayerApp.apk")
	}
}
