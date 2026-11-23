package webpush

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// receiver is a fake browser: a P-256 key pair and auth secret.
type receiver struct {
	priv *ecdh.PrivateKey
	auth []byte
}

func newReceiver(t *testing.T) (*receiver, Subscription) {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	return &receiver{priv: priv, auth: auth}, Subscription{
		Endpoint: "https://push.example.test/send/abc",
		P256dh:   b64.EncodeToString(priv.PublicKey().Bytes()),
		Auth:     b64.EncodeToString(auth),
	}
}

// decrypt is the user-agent side of RFC 8291, used to prove Encrypt is
// interoperable rather than merely self-consistent.
func (r *receiver) decrypt(t *testing.T, body []byte) []byte {
	t.Helper()
	if len(body) < 21 {
		t.Fatalf("body too short: %d", len(body))
	}
	salt := body[:16]
	if rs := binary.BigEndian.Uint32(body[16:20]); rs != recordSize {
		t.Fatalf("record size = %d", rs)
	}
	idlen := int(body[20])
	asPubRaw := body[21 : 21+idlen]
	ciphertext := body[21+idlen:]
	asPub, err := ecdh.P256().NewPublicKey(asPubRaw)
	if err != nil {
		t.Fatalf("keyid: %v", err)
	}
	shared, err := r.priv.ECDH(asPub)
	if err != nil {
		t.Fatal(err)
	}
	cek, nonce, err := deriveKeys(shared, r.auth, r.priv.PublicKey().Bytes(), asPubRaw, salt)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if plain[len(plain)-1] != 0x02 {
		t.Fatalf("missing last-record delimiter")
	}
	return plain[:len(plain)-1]
}

func testKeys(t *testing.T) Keys {
	t.Helper()
	pub, priv, err := GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}
	return Keys{Public: pub, Private: priv, Subject: "mailto:coach@example.test"}
}

func TestGenerateAndParseKeys(t *testing.T) {
	k := testKeys(t)
	if _, err := ParseKeys(k); err != nil {
		t.Fatalf("parse: %v", err)
	}
	raw, _ := decodeB64(k.Public)
	if len(raw) != 65 || raw[0] != 0x04 {
		t.Fatalf("public key should be a 65-byte uncompressed point, got %d bytes", len(raw))
	}
	other := testKeys(t)
	if _, err := ParseKeys(Keys{Public: other.Public, Private: k.Private}); err == nil {
		t.Fatal("mismatched halves should fail")
	}
}

func TestEncryptRoundTrip(t *testing.T) {
	r, sub := newReceiver(t)
	payload := []byte(`{"title":"Rest done","body":"Next set"}`)
	body, err := Encrypt(sub, payload)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.decrypt(t, body); string(got) != string(payload) {
		t.Fatalf("decrypted %q", got)
	}
	if _, err := Encrypt(sub, make([]byte, MaxPayload+1)); err == nil {
		t.Fatal("oversized payload should fail")
	}
	if _, err := Encrypt(Subscription{Endpoint: sub.Endpoint, P256dh: sub.P256dh, Auth: "AAAA"}, payload); err == nil {
		t.Fatal("short auth secret should fail")
	}
}

func verifyVAPID(t *testing.T, header string, k Keys, wantAud string) {
	t.Helper()
	if !strings.HasPrefix(header, "vapid t=") || !strings.Contains(header, ", k="+k.Public) {
		t.Fatalf("header = %q", header)
	}
	jwt := strings.TrimPrefix(strings.SplitN(header, ", k=", 2)[0], "vapid t=")
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("jwt parts = %d", len(parts))
	}
	claimsRaw, _ := b64.DecodeString(parts[1])
	var claims struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(claimsRaw, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Aud != wantAud || claims.Sub != k.Subject {
		t.Fatalf("claims = %+v", claims)
	}
	if ttl := time.Until(time.Unix(claims.Exp, 0)); ttl <= 0 || ttl > 24*time.Hour {
		t.Fatalf("exp out of range: %v", ttl)
	}
	sig, _ := b64.DecodeString(parts[2])
	pubRaw, _ := decodeB64(k.Public)
	pub, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), pubRaw)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	rr := new(big.Int).SetBytes(sig[:32])
	ss := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(pub, digest[:], rr, ss) {
		t.Fatal("VAPID signature does not verify")
	}
}

func TestVAPIDHeader(t *testing.T) {
	k := testKeys(t)
	h, err := VAPIDHeader("https://fcm.googleapis.com/fcm/send/xyz", k, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	verifyVAPID(t, h, k, "https://fcm.googleapis.com")
	if _, err := VAPIDHeader("not a url", k, time.Now()); err == nil {
		t.Fatal("relative endpoint should fail")
	}
}

func TestSend(t *testing.T) {
	k := testKeys(t)
	r, sub := newReceiver(t)
	var got []byte
	status := http.StatusCreated
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Content-Encoding") != "aes128gcm" || req.Header.Get("TTL") != "60" || req.Header.Get("Urgency") != "high" {
			t.Errorf("headers = %v", req.Header)
		}
		verifyVAPID(t, req.Header.Get("Authorization"), k, "https://"+req.Host)
		got, _ = io.ReadAll(req.Body)
		w.WriteHeader(status)
	}))
	defer srv.Close()
	sub.Endpoint = srv.URL + "/push/1"

	msg := Message{Payload: []byte("hello"), TTL: 60, Urgency: "high"}
	if err := Send(context.Background(), srv.Client(), sub, k, msg); err != nil {
		t.Fatalf("send: %v", err)
	}
	if plain := r.decrypt(t, got); string(plain) != "hello" {
		t.Fatalf("payload = %q", plain)
	}

	status = http.StatusGone
	if err := Send(context.Background(), srv.Client(), sub, k, msg); !errors.Is(err, ErrGone) {
		t.Fatalf("410 should be ErrGone, got %v", err)
	}
	status = http.StatusBadRequest
	if err := Send(context.Background(), srv.Client(), sub, k, msg); err == nil || errors.Is(err, ErrGone) {
		t.Fatalf("400 should be a plain error, got %v", err)
	}
}

// TestEncryptRFC8291Vector checks the exact body from RFC 8291 Appendix A.
// Note the RFC example uses a 4096-byte record size, as Encrypt does.
func TestEncryptRFC8291Vector(t *testing.T) {
	asPrivRaw, _ := decodeB64("yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw")
	asPriv, err := ecdh.P256().NewPrivateKey(asPrivRaw)
	if err != nil {
		t.Fatal(err)
	}
	salt, _ := decodeB64("DGv6ra1nlYgDCS1FRnbzlw")
	sub := Subscription{
		Endpoint: "https://push.example.net/push/JzLQ3raZJfFBR0aqvOMsLrt54w4rJUsV",
		P256dh:   "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4",
		Auth:     "BTBZMqHH6r4Tts7J_aSIgg",
	}
	body, err := encrypt(sub, []byte("When I grow up, I want to be a watermelon"), asPriv, salt)
	if err != nil {
		t.Fatal(err)
	}
	want := "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPTpK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"
	if got := b64.EncodeToString(body); got != want {
		t.Fatalf("body mismatch\n got %s\nwant %s", got, want)
	}
}
