Unpacks cassette images from Walkman NW-A100 firmware, then crops, resizes and rotates them for smaller screen.

### Usage

```shell
Usage of ./cassetteunpacker-linux-amd64:
  -c	remove intermediate files (default true)
  -f string
    	path to firmware file (default "NW-A100_0003_V4_04_00_NW_WM_FW.UPG")
  -k string
    	firmware decryption key (default "o0W0mihblfJOPtNQPN8Pc2hLKiTROL5MVERN9OmmkkMNZO2P")
```

### Build

```shell
make
```

Requirements for protobuf definitions:
- [protoc](https://github.com/protocolbuffers/protobuf/releases/latest)
- [protoc-gen-go](https://protobuf.dev/reference/go/go-generated/)

### Run

Requirements:
  - `e2tools` package to get file from ext2 image file, `e2cp` binary in particular [GitHub](https://github.com/e2tools/e2tools)

```shell
$ ./cassetteunpacker-linux-amd64 
2024/11/14 08:41:43 INFO decrypting firmware
2024/11/14 08:41:44 INFO getting payload
2024/11/14 08:41:44 INFO getting metadata
2024/11/14 08:41:44 INFO dumping partition "partition name"=system
2024/11/14 08:42:23 INFO dumped partition "partition name"=system
2024/11/14 08:42:23 INFO getting apk
2024/11/14 08:42:28 INFO extracting images
2024/11/14 08:42:32 INFO putting files into ./res
2024/11/14 08:42:32 INFO cleaning up

$ tree --filelimit=5 --noreport ./res/
./res/
├── reel
│   ├── chf  [57 entries exceeds filelimit, not opening dir]
│   ├── metal_master  [57 entries exceeds filelimit, not opening dir]
│   └── other  [57 entries exceeds filelimit, not opening dir]
└── tape  [9 entries exceeds filelimit, not opening dir]

```

### See also:
  - https://github.com/notcbw/2019_android_walkman
  - https://github.com/google/ota-analyzer
  - https://github.com/vm03/payload_dumper