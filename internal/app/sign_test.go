package app

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Arameair/identity/internal/pairing"
	"github.com/Arameair/identity/internal/profile"
)

// Test-side implementation of the identity/2 request signing that Thought
// Processor uses. The secret itself is never put on the request.
var testPIDs sync.Map // secret -> processor_id

func signTest(req *http.Request, secret, body string) string {
	pid, ok := testPIDs.Load(secret)
	if !ok {
		pid = "tp_not_paired"
	}
	rk, _ := pairing.DeriveKeys(secret)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := profile.RandHex(12)
	req.Header.Set(ProcessorIDHeader, pid.(string))
	req.Header.Set(TimeHeader, ts)
	req.Header.Set(NonceHeader, nonce)
	req.Header.Set(AuthHeader, pairing.RequestMAC(rk, req.Method, req.URL.RequestURI(), ts, nonce, []byte(body)))
	return nonce
}

func proofOK(secret, nonce string, status int, raw []byte, proof string) bool {
	_, sk := pairing.DeriveKeys(secret)
	return pairing.ResponseProof(sk, nonce, status, raw) == proof
}

func respKeyOf(secret string) []byte { _, sk := pairing.DeriveKeys(secret); return sk }
