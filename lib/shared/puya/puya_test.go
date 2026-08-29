package puya

import (
	"testing"

	"github.com/burgrp/bleriot/lib/shared/inventory"
)

func TestProfiles(t *testing.T) {
	cases := []struct {
		profile inventory.Chip
		name    string
		pack    string
	}{
		{PY32F002Ax5, "py32f002ax5", "PY32F002A"},
		{PY32F002Bx5, "py32f002bx5", "PY32F002B"},
		{PY32F003x4, "py32f003x4", "PY32F003"},
		{PY32F003x6, "py32f003x6", "PY32F003"},
		{PY32F003x7, "py32f003x7", "PY32F003"},
		{PY32F003x8, "py32f003x8", "PY32F003"},
		{PY32F030x4, "py32f030x4", "PY32F030"},
		{PY32F030x6, "py32f030x6", "PY32F030"},
		{PY32F030x7, "py32f030x7", "PY32F030"},
		{PY32F030x8, "py32f030x8", "PY32F030"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := inventory.Chip{
				Name: tc.name, TinygoTarget: tc.name, PyocdTarget: tc.name, CmsisPack: tc.pack,
			}
			if tc.profile != want {
				t.Fatalf("profile = %+v, want %+v", tc.profile, want)
			}
		})
	}
}

func TestMemoryMapConstants(t *testing.T) {
	constants := []struct {
		name string
		got  uint32
		want uint32
	}{
		{"FlashBase", FlashBase, 0x08000000},
		{"SRAMBase", SRAMBase, 0x20000000},
		{"F002AUIDAddr", F002AUIDAddr, 0x1FFF0E00},
		{"F002AOptionBytesBase", F002AOptionBytesBase, 0x1FFF0E80},
		{"F002AFactoryConfigBase", F002AFactoryConfigBase, 0x1FFF0F00},
		{"F002ABootloaderBase", F002ABootloaderBase, 0x1FFF0000},
		{"F002BUIDAddr", F002BUIDAddr, 0x1FFF0000},
		{"F002BOptionBytesBase", F002BOptionBytesBase, 0x1FFF0080},
		{"F002BFactoryConfig0Base", F002BFactoryConfig0Base, 0x1FFF0100},
		{"F002BFactoryConfig1Base", F002BFactoryConfig1Base, 0x1FFF0180},
		{"F002BUserOTPBase", F002BUserOTPBase, 0x1FFF0280},
		{"F003UIDAddr", F003UIDAddr, 0x1FFF0E00},
		{"F003OptionBytesBase", F003OptionBytesBase, 0x1FFF0E80},
		{"F003FactoryConfigBase", F003FactoryConfigBase, 0x1FFF0F00},
		{"F003BootloaderBase", F003BootloaderBase, 0x1FFF0000},
		{"F030UIDAddr", F030UIDAddr, 0x1FFF0E00},
		{"F030OptionBytesBase", F030OptionBytesBase, 0x1FFF0E80},
		{"F030FactoryConfigBase", F030FactoryConfigBase, 0x1FFF0F00},
		{"F030BootloaderBase", F030BootloaderBase, 0x1FFF0000},
	}
	for _, tc := range constants {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("%s = %#x, want %#x", tc.name, tc.got, tc.want)
			}
		})
	}
}
