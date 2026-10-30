//go:build linux

// Copyright 2026 The sing-box Authors
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oomprofile

import (
	"bytes"
	"strconv"
	"strings"
)

// parseProcSelfMaps extracts executable mappings from Linux's
// /proc/self/maps. It is based on the equivalent runtime/pprof helper, kept
// locally because that helper is not an externally linkable runtime symbol.
func parseProcSelfMaps(data []byte, addMapping func(lo, hi, offset uint64, file, buildID string)) {
	for len(data) > 0 {
		line, remainder, _ := bytes.Cut(data, []byte("\n"))
		data = remainder
		next := func() []byte {
			field, rest, _ := bytes.Cut(line, []byte(" "))
			line = bytes.TrimLeft(rest, " ")
			return field
		}

		addresses := next()
		loString, hiString, ok := strings.Cut(string(addresses), "-")
		if !ok {
			continue
		}
		lo, err := strconv.ParseUint(loString, 16, 64)
		if err != nil {
			continue
		}
		hi, err := strconv.ParseUint(hiString, 16, 64)
		if err != nil {
			continue
		}
		permissions := next()
		if len(permissions) < 4 || permissions[2] != 'x' {
			continue
		}
		offset, err := strconv.ParseUint(string(next()), 16, 64)
		if err != nil {
			continue
		}
		next() // device
		inode := next()
		if line == nil {
			continue
		}
		file := string(line)

		const deletedSuffix = " (deleted)"
		file = strings.TrimSuffix(file, deletedSuffix)
		if len(inode) == 1 && inode[0] == '0' && file == "" {
			// Skip unpopulated huge-page mappings, but retain named mappings
			// such as [vdso] and [vsyscall].
			continue
		}
		buildID, _ := elfBuildID(file)
		addMapping(lo, hi, offset, file, buildID)
	}
}
