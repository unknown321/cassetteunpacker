
PRODUCT=cassetteunpacker
GOOS=linux
GOARCH=$(shell uname -m)
GOARM=
NAME=$(PRODUCT)-$(GOOS)-$(GOARCH)$(GOARM)

ifeq ($(GOARCH),x86_64)
	override GOARCH=amd64
endif

UPDATE_METADATA_URL=https://android.googlesource.com/platform/system/update_engine/+/4ff049167edfe381966619f3123681f2e6c02138/update_metadata.proto?format=TEXT
UPDATE_METADATA_SHA256=09da1556e3edb9197ca88103b22ea07230a634c004605d4aa1efee6a6ed6e60d

update_metadata.proto:
	curl --silent -L $(UPDATE_METADATA_URL) | base64 -d > update_metadata.proto
	echo "$(UPDATE_METADATA_SHA256)  update_metadata.proto" | sha256sum --check --status

metadata/update_metadata.pb.go:
	protoc -I=. --go_out=. --go_opt=Mupdate_metadata.proto=/metadata update_metadata.proto

$(NAME): update_metadata.proto metadata/update_metadata.pb.go
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -a \
		-ldflags "-w -s" \
		-trimpath \
		-o $(NAME)

build: $(NAME)

all: build

clean:
	rm -rfv $(PRODUCT)-*

veryclean: clean
	rm -rf system.ext2 HighResMediaPlayerApp.apk res update_metadata.proto metadata/update_metadata.pb.go


.PHONY: test
.DEFAULT_GOAL := all