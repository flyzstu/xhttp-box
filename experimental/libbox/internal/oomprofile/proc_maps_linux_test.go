//go:build linux

package oomprofile

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseProcSelfMaps(t *testing.T) {
	input := []byte("1000-2000 r-xp 00000010 00:01 1 /usr/bin/test\n" +
		"2000-3000 rw-p 00000000 00:01 1 /usr/bin/test\n" +
		"3000-4000 r-xp 00000020 00:00 0 \n" +
		"4000-5000 r-xp 00000030 00:00 0 [vdso]\n" +
		"5000-6000 r-xp 00000040 00:01 1 /tmp/deleted (deleted)\n")
	type mapping struct {
		lo, hi, offset uint64
		file           string
	}
	var mappings []mapping
	parseProcSelfMaps(input, func(lo, hi, offset uint64, file, _ string) {
		mappings = append(mappings, mapping{lo, hi, offset, file})
	})
	require.Equal(t, []mapping{
		{0x1000, 0x2000, 0x10, "/usr/bin/test"},
		{0x4000, 0x5000, 0x30, "[vdso]"},
		{0x5000, 0x6000, 0x40, "/tmp/deleted"},
	}, mappings)
}
