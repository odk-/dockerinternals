package registry

import (
	"testing"
)

func TestImageParsing(t *testing.T) {
	SetDefaultRegistry("registry-1.docker.io")

	var cases = map[string]Image{
		"busybox":                             {"registry-1.docker.io", "library/busybox", "latest", ""},
		"odk/busybox":                         {"registry-1.docker.io", "odk/busybox", "latest", ""},
		"busybox:v1":                          {"registry-1.docker.io", "library/busybox", "v1", ""},
		"odk/busybox:v1":                      {"registry-1.docker.io", "odk/busybox", "v1", ""},
		"somehost.domain/busybox":             {"somehost.domain", "busybox", "latest", ""},
		"somehost.domain/odk/busybox":         {"somehost.domain", "odk/busybox", "latest", ""},
		"somehost.domain/busybox:v1":          {"somehost.domain", "busybox", "v1", ""},
		"somehost.domain/odk/busybox:v1":      {"somehost.domain", "odk/busybox", "v1", ""},
		"somehost.domain:5000/odk/busybox:v1": {"somehost.domain:5000", "odk/busybox", "v1", ""},
		"somehost.domain:5000/odk/busybox":    {"somehost.domain:5000", "odk/busybox", "latest", ""},
	}

	var resp *Image
	var err error
	for param, ans := range cases {
		resp, err = ParseImageName(param)
		if err != nil {
			t.Error("Got error: ", err)
		}
		if *resp != ans {
			t.Errorf("For %s expecting %v, got %v", param, ans, resp)
		}
	}
}
