// Package webpush sends Web Push messages (RFC 8030) with VAPID
// authentication (RFC 8292) and aes128gcm payload encryption (RFC 8291).
//
// It uses only the standard library so the static, CGO-free binary does not
// pick up a new dependency. The package knows nothing about the database:
// callers resolve keys and subscriptions and hand them in.
package webpush

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// recordSize is the aes128gcm record size advertised in the header. A
// single record carries the whole payload, so it only needs to exceed it.
const recordSize = 4096

// MaxPayload is the largest plaintext that fits in one record (record size
// minus the 16-byte tag and the 1-byte padding delimiter). Push services
// guarantee at least 4096 bytes of body including the header, so keep
// payloads well under this.
const MaxPayload = 3000

// Keys is a VAPID application-server key pair, both halves base64url
// encoded without padding: Public is the 65-byte uncompressed P-256 point
// (what the browser's applicationServerKey takes) and Private is the
// 32-byte scalar.
type Keys struct {
	Public  string
	Private string
	// Subject is the VAPID "sub" claim: a mailto: or https: contact URI.
	Subject string
}

// Subscription is one browser PushSubscription.
type Subscription struct {
	Endpoint string
	P256dh   string // base64url receiver public key
	Auth     string // base64url 16-byte auth secret
}

// Message is one push to deliver.
type Message struct {
	Payload []byte
	// TTL is how long, in seconds, the push service may hold the message
	// for an offline device. Zero means deliver now or drop.
	TTL int
	// Urgency is very-low, low, normal or high (RFC 8030 §5.3). Empty
	// leaves the header off.
	Urgency string
	// Topic lets a newer message replace an undelivered older one.
	Topic string
}

// ErrGone reports that the push service says the subscription no longer
// exists (404 or 410). The caller should delete it.
var ErrGone = errors.New("webpush: subscription expired or unsubscribed")

var b64 = base64.RawURLEncoding

// decodeB64 accepts base64url with or without padding, and standard
// base64 too, since browsers and libraries disagree.
func decodeB64(s string) ([]byte, error) {
	s = strings.TrimRight(strings.TrimSpace(s), "=")
	s = strings.NewReplacer("+", "-", "/", "_").Replace(s)
	return b64.DecodeString(s)
}

// GenerateKeys creates a new VAPID key pair.
func GenerateKeys() (public, private string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("webpush: generate key: %w", err)
	}
	priv, err := key.Bytes()
	if err != nil {
		return "", "", fmt.Errorf("webpush: encode private key: %w", err)
	}
	pub, err := key.PublicKey.Bytes()
	if err != nil {
		return "", "", fmt.Errorf("webpush: encode public key: %w", err)
	}
	return b64.EncodeToString(pub), b64.EncodeToString(priv), nil
}

// ParseKeys checks that a key pair decodes and that the halves match.
func ParseKeys(k Keys) (*ecdsa.PrivateKey, error) {
	raw, err := decodeB64(k.Private)
	if err != nil {
		return nil, fmt.Errorf("webpush: private key is not base64url: %w", err)
	}
	priv, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), raw)
	if err != nil {
		return nil, fmt.Errorf("webpush: private key: %w", err)
	}
	pubRaw, err := decodeB64(k.Public)
	if err != nil {
		return nil, fmt.Errorf("webpush: public key is not base64url: %w", err)
	}
	want, err := priv.PublicKey.Bytes()
	if err != nil {
		return nil, fmt.Errorf("webpush: public key: %w", err)
	}
	if !bytes.Equal(pubRaw, want) {
		return nil, errors.New("webpush: public key does not match private key")
	}
	return priv, nil
}

// Encrypt encrypts plaintext for the subscription using aes128gcm
// (RFC 8291). The result is the full request body: header then one record.
func Encrypt(sub Subscription, plaintext []byte) ([]byte, error) {
	asPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("webpush: ephemeral key: %w", err)
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("webpush: salt: %w", err)
	}
	return encrypt(sub, plaintext, asPriv, salt)
}

// encrypt takes the ephemeral key and salt so the RFC 8291 test vector
// can pin them.
func encrypt(sub Subscription, plaintext []byte, asPriv *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	if len(plaintext) > MaxPayload {
		return nil, fmt.Errorf("webpush: payload is %d bytes, limit %d", len(plaintext), MaxPayload)
	}
	uaPubRaw, err := decodeB64(sub.P256dh)
	if err != nil {
		return nil, fmt.Errorf("webpush: p256dh: %w", err)
	}
	authSecret, err := decodeB64(sub.Auth)
	if err != nil {
		return nil, fmt.Errorf("webpush: auth: %w", err)
	}
	if len(authSecret) != 16 {
		return nil, fmt.Errorf("webpush: auth secret is %d bytes, want 16", len(authSecret))
	}
	uaPub, err := ecdh.P256().NewPublicKey(uaPubRaw)
	if err != nil {
		return nil, fmt.Errorf("webpush: p256dh: %w", err)
	}
	asPubRaw := asPriv.PublicKey().Bytes()
	shared, err := asPriv.ECDH(uaPub)
	if err != nil {
		return nil, fmt.Errorf("webpush: ecdh: %w", err)
	}
	cek, nonce, err := deriveKeys(shared, authSecret, uaPubRaw, asPubRaw, salt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	// One record, so it is the last: pad with the 0x02 delimiter only.
	padded := append(append([]byte{}, plaintext...), 0x02)
	sealed := gcm.Seal(nil, nonce, padded, nil)

	var body bytes.Buffer
	body.Write(salt)
	_ = binary.Write(&body, binary.BigEndian, uint32(recordSize))
	body.WriteByte(byte(len(asPubRaw)))
	body.Write(asPubRaw)
	body.Write(sealed)
	return body.Bytes(), nil
}

// deriveKeys runs the RFC 8291 §3.3–3.4 key schedule and returns the
// content-encryption key and nonce.
func deriveKeys(shared, authSecret, uaPub, asPub, salt []byte) (cek, nonce []byte, err error) {
	prkKey, err := hkdf.Extract(sha256.New, shared, authSecret)
	if err != nil {
		return nil, nil, err
	}
	keyInfo := "WebPush: info\x00" + string(uaPub) + string(asPub)
	ikm, err := hkdf.Expand(sha256.New, prkKey, keyInfo, 32)
	if err != nil {
		return nil, nil, err
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, nil, err
	}
	cek, err = hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, nil, err
	}
	nonce, err = hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, nil, err
	}
	return cek, nonce, nil
}

// VAPIDHeader builds the Authorization header value for a push to
// endpoint: "vapid t=<jwt>, k=<public key>".
func VAPIDHeader(endpoint string, keys Keys, now time.Time) (string, error) {
	priv, err := ParseKeys(keys)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("webpush: endpoint %q is not an absolute URL", endpoint)
	}
	subject := keys.Subject
	if subject == "" {
		subject = "mailto:admin@localhost"
	}
	header, _ := json.Marshal(map[string]string{"typ": "JWT", "alg": "ES256"})
	claims, _ := json.Marshal(map[string]any{
		"aud": u.Scheme + "://" + u.Host,
		// Push services reject an exp more than 24h out; 12h is the norm.
		"exp": now.Add(12 * time.Hour).Unix(),
		"sub": subject,
	})
	signing := b64.EncodeToString(header) + "." + b64.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	if err != nil {
		return "", fmt.Errorf("webpush: sign: %w", err)
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	jwt := signing + "." + b64.EncodeToString(sig)
	return "vapid t=" + jwt + ", k=" + strings.TrimRight(keys.Public, "="), nil
}

// Send encrypts and posts one message. It returns ErrGone when the push
// service reports the subscription is gone, and an error for any other
// non-2xx answer.
func Send(ctx context.Context, client *http.Client, sub Subscription, keys Keys, msg Message) error {
	if client == nil {
		client = http.DefaultClient
	}
	body, err := Encrypt(sub, msg.Payload)
	if err != nil {
		return err
	}
	auth, err := VAPIDHeader(sub.Endpoint, keys, time.Now())
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webpush: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("TTL", strconv.Itoa(max(msg.TTL, 0)))
	req.Header.Set("Authorization", auth)
	if msg.Urgency != "" {
		req.Header.Set("Urgency", msg.Urgency)
	}
	if msg.Topic != "" {
		req.Header.Set("Topic", msg.Topic)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webpush: post: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return ErrGone
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return fmt.Errorf("webpush: push service answered %d", resp.StatusCode)
	}
	return nil
}
