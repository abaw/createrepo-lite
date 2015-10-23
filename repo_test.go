package main

import (
	"path/filepath"
	"testing"
)

func shouldEqualStr(t *testing.T, msg string, value string, expected string) {
	if value != expected {
		t.Error(msg, value, "!=", expected)
	}
}

func shouldEqualU32(t *testing.T, msg string, value uint32, expected uint32) {
	if value != expected {
		t.Error(msg, value, "!=", expected)
	}
}

func shouldEqualU64(t *testing.T, msg string, value uint64, expected uint64) {
	if value != expected {
		t.Error(msg, value, "!=", expected)
	}
}

func shouldBeValidAndEqualStr(t *testing.T, msg string, value *string, expected string) {
	if value == nil {
		t.Error(msg, "nil")
	} else {
		shouldEqualStr(t, msg, *value, expected)
	}
}

func TestPackageInfo(t *testing.T) {
	ts := newTS()
	defer ts.close()

	info, err := ts.parsePackageInfo("openssl.rpm")
	if err != nil {
		t.Fatal("parsePackageInfo(openssl.rpm) failed:", err.Error())
	}

	p, _ := filepath.Abs("openssl.rpm")
	shouldEqualStr(t, "wrong value for info.path:", info.path, p)
	shouldEqualStr(t, "wrong value for info.checksum:", info.checksum, "6c41a21d88d83691e9ff90fe1612a72f6f63bb8ebaaf8442c00c3cfdfd177e22")
	shouldEqualStr(t, "info.checksumType", info.checksumType, "sha256")
	shouldEqualU64(t, "info.fileSize", info.fileSize, 1589496)
	shouldEqualU64(t, "info.headerStart", info.headerStart, 1384)
	shouldEqualU64(t, "info.headerEnd", info.headerEnd, 61140)
	shouldEqualStr(t, "info.rpmName", info.rpmName, "openssl")
	shouldEqualStr(t, "info.rpmArch", info.rpmArch, "x86_64")
	shouldEqualStr(t, "info.rpmVersion", info.rpmVersion, "1.0.1e")
	shouldEqualStr(t, "info.rpmEpoch", info.rpmEpoch, "0")
	shouldEqualStr(t, "info.rpmRelease", info.rpmRelease, "30.el6_6.5")
	shouldEqualStr(t, "info.rpmSummary", info.rpmSummary, "A general purpose cryptography library with TLS implementation")
	shouldEqualStr(t, "info.rpmDescription", info.rpmDescription, `The OpenSSL toolkit provides support for secure communications between
machines. OpenSSL includes a certificate management tool and shared
libraries which provide various cryptographic algorithms and
protocols.`)
	shouldBeValidAndEqualStr(t, "info.rpmUrl", info.rpmUrl, "http://www.openssl.org/")
	shouldEqualU32(t, "info.rpmBuildTime", info.rpmBuildTime, 1421775236)
	shouldBeValidAndEqualStr(t, "info.rpmLicense", info.rpmLicense, "OpenSSL")
	shouldBeValidAndEqualStr(t, "info.rpmVendor", info.rpmVendor, "CentOS")
	shouldBeValidAndEqualStr(t, "info.rpmGroup", info.rpmGroup, "System Environment/Libraries")
	shouldBeValidAndEqualStr(t, "info.rpmBuildHost", info.rpmBuildHost, "c6b8.bsys.dev.centos.org")
	shouldBeValidAndEqualStr(t, "info.rpmSourceRpm", info.rpmSourceRpm, "openssl-1.0.1e-30.el6_6.5.src.rpm")
	shouldBeValidAndEqualStr(t, "info.rpmPackager", info.rpmPackager, "CentOS BuildSystem <http://bugs.centos.org>")
	shouldEqualU64(t, "info.rpmInstallSize", info.rpmInstallSize, 4222195)
	shouldEqualU64(t, "info.rpmArchiveSize", info.rpmArchiveSize, 4238188)
}
