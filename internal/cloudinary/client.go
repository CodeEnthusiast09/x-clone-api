package cloudinary

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Client struct {
	cloudName  string
	apiKey     string
	apiSecret  string
	httpClient *http.Client
}

func NewClient(cloudName, apiKey, apiSecret string) *Client {
	return &Client{
		cloudName:  cloudName,
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type UploadSignature struct {
	CloudName    string       `json:"cloudName"`
	UploadParams UploadParams `json:"uploadParams"`
}

// UploadParams holds the signed fields the mobile client posts to
// `https://api.cloudinary.com/v1_1/<cloud_name>/image/upload`.
// JSON tags use snake_case to match Cloudinary's expected form fields.
type UploadParams struct {
	Timestamp    int64  `json:"timestamp"`
	Signature    string `json:"signature"`
	APIKey       string `json:"api_key"`
	UploadPreset string `json:"upload_preset"`
	MaxBytes     int64  `json:"max_bytes"`
	PublicID     string `json:"public_id,omitempty"`
}

// Sign returns a short-lived signed upload token for direct mobile uploads.
// Cloudinary enforces maxBytes and (if provided) publicID server-side; the mobile
// client cannot change either without invalidating the signature. Pin publicID
// when callers need owner-scoped asset paths for later authorization checks.
func (c *Client) Sign(uploadPreset string, maxBytes int64, publicID string) *UploadSignature {
	ts := time.Now().Unix()
	params := map[string]string{
		"max_bytes":     fmt.Sprintf("%d", maxBytes),
		"timestamp":     fmt.Sprintf("%d", ts),
		"upload_preset": uploadPreset,
	}
	if publicID != "" {
		params["public_id"] = publicID
	}
	return &UploadSignature{
		CloudName: c.cloudName,
		UploadParams: UploadParams{
			Timestamp:    ts,
			Signature:    c.sign(params),
			APIKey:       c.apiKey,
			UploadPreset: uploadPreset,
			MaxBytes:     maxBytes,
			PublicID:     publicID,
		},
	}
}

// Destroy deletes an asset by its public_id. Idempotent: a "not found" result
// is treated as success so retries are safe.
func (c *Client) Destroy(ctx context.Context, publicID string) error {
	if publicID == "" {
		return nil
	}

	ts := time.Now().Unix()
	signed := map[string]string{
		"public_id": publicID,
		"timestamp": fmt.Sprintf("%d", ts),
	}
	sig := c.sign(signed)

	form := url.Values{}
	form.Set("public_id", publicID)
	form.Set("timestamp", fmt.Sprintf("%d", ts))
	form.Set("api_key", c.apiKey)
	form.Set("signature", sig)

	endpoint := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/destroy", c.cloudName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("cloudinary destroy: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cloudinary destroy: http: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("cloudinary destroy: decode response: %w", err)
	}

	if body.Result != "ok" && body.Result != "not found" {
		return fmt.Errorf("cloudinary destroy: unexpected result %q (status %d)", body.Result, resp.StatusCode)
	}
	return nil
}

var versionPrefix = regexp.MustCompile(`^v\d+/`)

// PublicIDFromURL extracts the Cloudinary public_id from a secure_url.
// Returns "" if the URL doesn't match the expected Cloudinary upload pattern.
//
// Example:
//
//	https://res.cloudinary.com/dxyz/image/upload/v1717488000/abc123xyz.jpg
//	→ "abc123xyz"
//
//	https://res.cloudinary.com/dxyz/image/upload/v1717488000/folder/abc.jpg
//	→ "folder/abc"
func PublicIDFromURL(secureURL string) string {
	u, err := url.Parse(secureURL)
	if err != nil {
		return ""
	}
	parts := strings.SplitN(u.Path, "/upload/", 2)
	if len(parts) != 2 {
		return ""
	}
	rest := versionPrefix.ReplaceAllString(parts[1], "")
	if i := strings.LastIndex(rest, "."); i > 0 {
		rest = rest[:i]
	}
	return rest
}

func (c *Client) sign(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+params[k])
	}

	h := sha1.New()
	h.Write([]byte(strings.Join(pairs, "&") + c.apiSecret))
	return fmt.Sprintf("%x", h.Sum(nil))
}
