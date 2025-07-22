package internal

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"path/filepath"
	"strconv"
	"strings"

	"go.uber.org/zap/zapcore"
)

type PathConfig struct {
	srcPath  string
	fileGlob string
	dstPath  string
	owner    uint32
	group    uint32
	mode     uint16
	isSet    uint8
}

func ParsePathConfig(spec string) (*PathConfig, error) {
	parts := strings.Split(spec, ":")
	if len(parts) == 0 || len(parts) > 6 {
		return nil, fmt.Errorf("invalid path config spec: %s", spec)
	}

	srcPath := parts[0]
	if !filepath.IsAbs(srcPath) {
		return nil, fmt.Errorf("source path must be absolute: %s", srcPath)
	}

	isSet := uint8(0)

	var fileGlob, dstPath string
	if len(parts) > 1 && parts[1] != "" && parts[1] != "*" {
		fileGlob = parts[1]
		if _, err := filepath.Match(fileGlob, ""); err != nil {
			return nil, fmt.Errorf("invalid file glob %q: %w", fileGlob, err)
		}
		isSet |= 0x1 // File glob is set
	}

	if len(parts) > 2 && parts[2] != "" {
		dstPath = parts[2]
		if !filepath.IsAbs(dstPath) {
			return nil, fmt.Errorf("destination path must be absolute: %s", dstPath)
		}
		isSet |= 0x2 // Destination path is set
	}

	var owner, group, mode uint64
	var err error
	if len(parts) > 3 && parts[3] != "" {
		owner, err = strconv.ParseUint(parts[3], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid owner %q", parts[3])
		}
		isSet |= 0x4 // Owner is set
	}

	if len(parts) > 4 && parts[4] != "" {
		group, err = strconv.ParseUint(parts[4], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid group %q", parts[4])
		}
		isSet |= 0x8 // Group is set
	}

	if len(parts) > 5 && parts[5] != "" {
		mode, err = strconv.ParseUint(parts[5], 8, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid mode %q", parts[5])
		}
		if mode > 0777 {
			return nil, fmt.Errorf("mode %04o is out of range, must be 0 to 0777", mode)
		}
		isSet |= 0x1 // Mode is set
	}

	if isSet&0x1c != 0 && isSet&0x2 == 0 {
		return nil, fmt.Errorf("can't set owner/group/mode without a destination path: %s", spec)
	}

	return &PathConfig{
		srcPath:  srcPath,
		dstPath:  dstPath,
		fileGlob: fileGlob,
		owner:    uint32(owner),
		group:    uint32(group),
		mode:     uint16(mode),
		isSet:    isSet,
	}, nil
}

func (pc *PathConfig) SrcPath() string {
	return pc.srcPath
}

func (pc *PathConfig) FileGlob() string {
	return pc.fileGlob
}

func (pc *PathConfig) DstPath() string {
	return pc.dstPath
}

func (pc *PathConfig) Owner() uint32 {
	return pc.owner
}

func (pc *PathConfig) Group() uint32 {
	return pc.group
}

func (pc *PathConfig) Mode() uint16 {
	return pc.mode
}

func (pc *PathConfig) HasFileGlob() bool {
	return pc.isSet&0x1 != 0
}

func (pc *PathConfig) HasDstPath() bool {
	return pc.isSet&0x2 != 0
}

func (pc *PathConfig) HasOwner() bool {
	return pc.isSet&0x4 != 0
}

func (pc *PathConfig) HasGroup() bool {
	return pc.isSet&0x8 != 0
}

func (pc *PathConfig) HasMode() bool {
	return pc.isSet&0x10 != 0
}

func (pc *PathConfig) String() string {
	parts := make([]string, 0, 6)
	parts = append(parts, pc.srcPath)
	if pc.isSet&0x1 != 0 {
		parts = append(parts, pc.fileGlob)
	} else if pc.isSet&0x1e != 0 {
		parts = append(parts, "*") // Default glob if no specific glob is set
	}
	if pc.isSet&0x2 != 0 {
		parts = append(parts, pc.dstPath)
	} else if pc.isSet&0x1c != 0 {
		parts = append(parts, "")
	}
	if pc.isSet&0x4 != 0 {
		parts = append(parts, strconv.FormatUint(uint64(pc.owner), 10))
	} else if pc.isSet&0x18 != 0 {
		parts = append(parts, "")
	}
	if pc.isSet&0x8 != 0 {
		parts = append(parts, strconv.FormatUint(uint64(pc.group), 10))
	} else if pc.isSet&0x10 != 0 {
		parts = append(parts, "")
	}
	if pc.isSet&0x10 != 0 {
		parts = append(parts, strconv.FormatUint(uint64(pc.mode), 8))
	}
	return strings.Join(parts, ":")
}

func (pc *PathConfig) ID() (string, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 64))
	buf.WriteString(pc.srcPath)
	if pc.HasFileGlob() {
		buf.WriteString(pc.fileGlob)
	}
	if pc.HasDstPath() {
		buf.WriteString(pc.dstPath)
	}

	hash := fnv.New64a()
	_, err := hash.Write(buf.Bytes())
	if err != nil {
		return "", err
	}

	buf.Reset()
	err = binary.Write(buf, binary.LittleEndian, hash.Sum64())
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(buf.Bytes()), nil
}

func (pc *PathConfig) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("srcPath", pc.srcPath)
	if pc.HasFileGlob() {
		enc.AddString("fileGlob", pc.fileGlob)
	} else {
		enc.AddString("fileGlob", "*")
	}
	if pc.HasDstPath() {
		enc.AddString("dstPath", pc.dstPath)
	}
	if pc.HasOwner() {
		enc.AddUint32("owner", pc.owner)
	}
	if pc.HasGroup() {
		enc.AddUint32("group", pc.group)
	}
	if pc.HasMode() {
		enc.AddString("mode", strconv.FormatUint(uint64(pc.mode), 8))
	}
	return nil
}
