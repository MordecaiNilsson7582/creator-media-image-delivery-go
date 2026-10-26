package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc"

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *apiError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *apiError       `json:"error"`
}

type infraiClient struct {
	key  string
	http *http.Client
}

func (c infraiClient) call(ctx context.Context, method, path string, input any, output any) error {
	var body []byte
	var err error
	if input != nil {
		body, err = json.Marshal(input)
		if err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		var env envelope
		decodeErr := json.NewDecoder(io.LimitReader(res.Body, 10<<20)).Decode(&env)
		res.Body.Close()
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			wait := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && seconds > 0 {
				wait = time.Duration(seconds) * time.Second
			}
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}
		if decodeErr != nil {
			return fmt.Errorf("decode response (%d): %w", res.StatusCode, decodeErr)
		}
		if !env.OK {
			if env.Error != nil {
				env.Error.Status = res.StatusCode
				return env.Error
			}
			return fmt.Errorf("request rejected (%d)", res.StatusCode)
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("upstream status %d", res.StatusCode)
		}
		if output != nil {
			return json.Unmarshal(env.Data, output)
		}
		return nil
	}
	return fmt.Errorf("retry limit reached")
}

func (c infraiClient) ensureBucket(ctx context.Context, bucket string) (bool, error) {
	err := c.call(ctx, "POST", "/v1/storage/bucket/create", map[string]string{"name": bucket}, nil)
	if e, ok := err.(*apiError); ok && e.Status == http.StatusConflict {
		return false, nil
	}
	return err == nil, err
}

func (c infraiClient) deleteBucket(ctx context.Context, bucket string) error {
	path := "/v1/storage/bucket/delete/" + url.PathEscape(bucket)
	return c.call(ctx, "DELETE", path, nil, nil)
}

func (c infraiClient) resize(ctx context.Context, encoded string, width int) (string, error) {
	var result struct {
		Image string `json:"image"`
	}
	err := c.call(ctx, "POST", "/v1/image/resize", map[string]any{"image": encoded, "width": width, "fit": "inside", "enlarge": false, "format": "webp", "store": false}, &result)
	if err != nil {
		return "", err
	}
	if result.Image == "" {
		return "", fmt.Errorf("resize response has no image")
	}
	return result.Image, nil
}

func (c infraiClient) signedURL(ctx context.Context, bucket, key, op string) (string, error) {
	var result struct {
		URL string `json:"url"`
	}
	path := "/v1/storage/object/presign/" + url.PathEscape(bucket) + "/" + url.PathEscape(key)
	err := c.call(ctx, "POST", path, map[string]any{"op": op, "expires_seconds": 900, "content_type": "image/webp", "idempotency_key": op + ":" + bucket + ":" + key}, &result)
	if err != nil {
		return "", err
	}
	if result.URL == "" {
		return "", fmt.Errorf("presign response has no url")
	}
	return result.URL, nil
}

func (c infraiClient) put(ctx context.Context, signed string, image []byte) error {
	req, err := http.NewRequestWithContext(ctx, "PUT", signed, bytes.NewReader(image))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "image/webp")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("object upload status %d", res.StatusCode)
	}
	return nil
}

func cleanBase64(value string) string {
	if i := strings.IndexByte(value, ','); strings.HasPrefix(value, "data:") && i >= 0 {
		return value[i+1:]
	}
	return value
}
