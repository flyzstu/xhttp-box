//go:build linux

// Copyright 2026 The sing-box Authors
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oomprofile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

var (
	errBadELF    = errors.New("malformed ELF binary")
	errNoBuildID = errors.New("no NT_GNU_BUILD_ID found in ELF binary")
)

// elfBuildID returns the GNU build ID without importing debug/elf and its
// transitive dependencies. It is derived from runtime/pprof's implementation.
func elfBuildID(file string) (string, error) {
	buffer := make([]byte, 256)
	input, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer input.Close()

	if _, err = input.ReadAt(buffer[:64], 0); err != nil {
		return "", err
	}
	if buffer[0] != 0x7F || buffer[1] != 'E' || buffer[2] != 'L' || buffer[3] != 'F' {
		return "", errBadELF
	}

	var byteOrder binary.ByteOrder
	switch buffer[5] {
	case 1:
		byteOrder = binary.LittleEndian
	case 2:
		byteOrder = binary.BigEndian
	default:
		return "", errBadELF
	}

	var sectionCount int
	var sectionOffset, sectionEntrySize int64
	switch buffer[4] {
	case 1:
		sectionOffset = int64(byteOrder.Uint32(buffer[32:]))
		sectionEntrySize = int64(byteOrder.Uint16(buffer[46:]))
		if sectionEntrySize != 40 {
			return "", errBadELF
		}
		sectionCount = int(byteOrder.Uint16(buffer[48:]))
	case 2:
		sectionOffset = int64(byteOrder.Uint64(buffer[40:]))
		sectionEntrySize = int64(byteOrder.Uint16(buffer[58:]))
		if sectionEntrySize != 64 {
			return "", errBadELF
		}
		sectionCount = int(byteOrder.Uint16(buffer[60:]))
	default:
		return "", errBadELF
	}

	for index := 0; index < sectionCount; index++ {
		if _, err = input.ReadAt(buffer[:sectionEntrySize], sectionOffset+int64(index)*sectionEntrySize); err != nil {
			return "", err
		}
		if byteOrder.Uint32(buffer[4:]) != 7 { // SHT_NOTE
			continue
		}
		var offset, end int64
		if sectionEntrySize == 40 {
			offset = int64(byteOrder.Uint32(buffer[16:]))
			end = offset + int64(byteOrder.Uint32(buffer[20:]))
		} else {
			offset = int64(byteOrder.Uint64(buffer[24:]))
			end = offset + int64(byteOrder.Uint64(buffer[32:]))
		}
		for offset < end {
			if _, err = input.ReadAt(buffer[:16], offset); err != nil {
				return "", err
			}
			nameSize := int(byteOrder.Uint32(buffer[0:]))
			descriptionSize := int(byteOrder.Uint32(buffer[4:]))
			noteType := int(byteOrder.Uint32(buffer[8:]))
			descriptionOffset := offset + int64(12+(nameSize+3)&^3)
			offset = descriptionOffset + int64((descriptionSize+3)&^3)
			if nameSize != 4 || noteType != 3 || buffer[12] != 'G' || buffer[13] != 'N' || buffer[14] != 'U' || buffer[15] != '\x00' {
				continue
			}
			if descriptionSize > len(buffer) {
				return "", errBadELF
			}
			if _, err = input.ReadAt(buffer[:descriptionSize], descriptionOffset); err != nil {
				return "", err
			}
			return fmt.Sprintf("%x", buffer[:descriptionSize]), nil
		}
	}
	return "", errNoBuildID
}
