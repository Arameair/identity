package redact

import (
	"strings"
	"testing"
)

// Canary values: if any of these survive String(), a secret could leak.
var canaries = map[string]string{
	"identity link":      "open http://127.0.0.1:7798/#key=0123456789abcdef0123456789abcdef0123456789abcdef now",
	"processor link":     "Processor: http://127.0.0.1:7860/#code=fedcba9876543210fedcba9876543210fedcba9876543210",
	"access token":       "got ghu_CANARYabcdefghijklmnopqrstuvwxyz0123 from GitHub",
	"classic token":      "token ghp_CANARY0123456789abcdefghijABCDEFGHIJ",
	"fine grained":       "github_pat_11CANARY0000000000_abcdefghijklmnopqrstuvwxyz",
	"refresh token":      "ghr_CANARYrefresh0123456789abcdefghijklmn",
	"auth header":        "Authorization: Bearer CANARYbearer-0123456789",
	"cookie header":      "Cookie: identity_b_session=CANARYcookievalue0123",
	"set cookie":         "Set-Cookie: tp_session=CANARYsetcookie; HttpOnly",
	"device code json":   `{"device_code":"CANARYdevice0123456789","user_code":"WDJB-MJHT"}`,
	"user code":          "Enter code WDJB-MJHT at github.com/login/device",
	"query code":         "callback?code=CANARYquerycode123&state=CANARYstate456",
	"client secret":      "client_secret=CANARYclientsecret0123",
	"private key":        "-----BEGIN RSA PRIVATE KEY-----\nCANARYkeymaterial\n-----END RSA PRIVATE KEY-----",
	"session cookie hex": "cookie value 9f8e7d6c5b4a39281706f5e4d3c2b1a09f8e7d6c5b4a39281706f5e4d3c2b1a0 seen",
	"password field":     `password: "CANARYpassword!"`,
}

var mustNotSurvive = []string{
	"0123456789abcdef0123456789abcdef0123456789abcdef",
	"fedcba9876543210fedcba9876543210fedcba9876543210",
	"ghu_CANARY", "ghp_CANARY", "github_pat_11CANARY", "ghr_CANARY",
	"CANARYbearer", "CANARYcookievalue", "CANARYsetcookie", "CANARYdevice",
	"WDJB-MJHT", "CANARYquerycode", "CANARYstate", "CANARYclientsecret",
	"CANARYkeymaterial", "9f8e7d6c5b4a39281706f5e4d3c2b1a09f8e7d6c5b4a3928", "CANARYpassword",
}

func TestCanariesNeverSurvive(t *testing.T) {
	for name, in := range canaries {
		out := String(in)
		for _, bad := range mustNotSurvive {
			if strings.Contains(out, bad) {
				t.Errorf("%s: %q survived redaction: %s", name, bad, out)
			}
		}
	}
}

func TestRegisteredSecret(t *testing.T) {
	Register("test", "plain-runtime-secret-xyz")
	defer Forget("plain-runtime-secret-xyz")
	if out := String("value=plain-runtime-secret-xyz end"); strings.Contains(out, "plain-runtime-secret-xyz") {
		t.Fatalf("registered secret leaked: %s", out)
	}
}

func TestUsefulEvidenceKept(t *testing.T) {
	in := "commit 0ba8bd52facedda1405897e9136d68ab184a2e32 port 7860 user Arameair 2026-10-25T03:44:00-05:00 req_a1b2c3d4e5f6"
	if out := String(in); out != in {
		t.Fatalf("non-secret evidence was altered:\n in: %s\nout: %s", in, out)
	}
}
