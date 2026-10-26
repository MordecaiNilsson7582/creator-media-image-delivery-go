package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var assetPart = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

type ingestRequest struct {
	Creator     string `json:"creator"`
	AssetID     string `json:"asset_id"`
	ImageBase64 string `json:"image_base64"`
	SourceWidth int    `json:"source_width"`
}

func assetKey(creator, assetID string) (string, error) {
	if !assetPart.MatchString(creator) || !assetPart.MatchString(assetID) {
		return "", errors.New("creator and asset_id must be 1-64 letters, digits, underscores or hyphens")
	}
	return creator + "/" + assetID + ".webp", nil
}

func deliveryWidth(source int) (int, error) {
	if source < 1 {
		return 0, errors.New("source_width must be positive")
	}
	if source > 1600 {
		return 1600, nil
	}
	return source, nil
}

type mediaService struct {
	client infraiClient
	bucket string
}

func (s mediaService) ingest(ctx context.Context, in ingestRequest) (string, error) {
	key, err := assetKey(in.Creator, in.AssetID)
	if err != nil {
		return "", err
	}
	width, err := deliveryWidth(in.SourceWidth)
	if err != nil {
		return "", err
	}
	if in.ImageBase64 == "" {
		return "", errors.New("image_base64 is required")
	}
	optimized, err := s.client.resize(ctx, in.ImageBase64, width)
	if err != nil {
		return "", err
	}
	image, err := base64.StdEncoding.DecodeString(cleanBase64(optimized))
	if err != nil {
		return "", fmt.Errorf("decode optimized image: %w", err)
	}
	signed, err := s.client.signedURL(ctx, s.bucket, key, "put")
	if err != nil {
		return "", err
	}
	if err := s.client.put(ctx, signed, image); err != nil {
		return "", err
	}
	return key, nil
}

func (s mediaService) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/jobs" {
		var input ingestRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 12<<20)).Decode(&input); err != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
		key, err := s.ingest(r.Context(), input)
		if err != nil {
			respondError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"key": key, "state": "ready"})
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/assets/") {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/assets/"), "/")
		if len(parts) != 2 || !strings.HasSuffix(parts[1], ".webp") {
			http.NotFound(w, r)
			return
		}
		key, err := assetKey(parts[0], strings.TrimSuffix(parts[1], ".webp"))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		link, err := s.client.signedURL(r.Context(), s.bucket, key, "get")
		if err != nil {
			respondError(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"key": key, "url": link})
		return
	}
	http.NotFound(w, r)
}

func respondError(w http.ResponseWriter, err error) {
	var remote *apiError
	if errors.As(err, &remote) {
		status := http.StatusBadGateway
		if remote.Status >= 400 && remote.Status < 500 {
			status = remote.Status
		}
		writeJSON(w, status, map[string]string{"error": remote.Code, "message": remote.Message})
		return
	}
	if strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "required") {
		http.Error(w, err.Error(), 400)
		return
	}
	http.Error(w, err.Error(), 502)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
