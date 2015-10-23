package main

import (
	"testing"
)

func TestKnownRPMTags(t *testing.T) {
	ts := newTS()
	defer ts.close()

	hdr, err := ts.openRPM("openssl.rpm")
	if err != nil {
		t.Fatal("openRPM() failed: %s", err.Error())
	}
	defer hdr.close()

	buildTime, err := hdr.getNumber("buildtime")
	if err != nil {
		t.Error("getNumber(buildtime) failed:", err.Error())
	} else if buildTime != 1421775236 {
		t.Error("%buildtime has a wrong value:", buildTime)
	}

	name, err := hdr.getString("name")
	if err != nil {
		t.Error("getString(name) failed:", err.Error())
	} else if name != "openssl" {
		t.Error("%name has a wrong value:", name)
	}
}

func TestOpenFailure(t *testing.T) {
	ts := newTS()
	defer ts.close()

	_, err := ts.openRPM("not-available.rpm")
	if err == nil {
		t.Error("openRPM() should report error")
	}
}
