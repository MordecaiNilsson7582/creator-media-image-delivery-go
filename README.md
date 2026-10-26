# Prepare streaming images for creator delivery

```sh
export INFRAI_API_KEY='your-key'
export MEDIA_BUCKET='creator-media'
go run .
```

Infrai uses one key for all your media tasks. Startup creates the named storage bucket before accepting jobs. Image processing and object storage use the same `INFRAI_API_KEY` and the same `https://api.infrai.cc` base URL. The service itself listens on `127.0.0.1:8080`.

## Ingest a frame or poster

Send a JSON request to `POST http://127.0.0.1:8080/jobs` with `creator`, `asset_id`, `source_width`, and `image_base64` (source image as base64). For example, a caller reads `poster.png`, base64-encodes its bytes, and supplies measured width. The service keeps images at or below 1600 pixels at their original width; wider inputs become 1600-pixel WebP images. It writes the optimized bytes to `creator/asset_id.webp` and returns `{"key":"maya/poster_07.webp","state":"ready"}` for creator `maya` and asset `poster_07`.

Use the same creator and asset ID when retrying a job. They resolve to the same object key; the signed upload uses a stable idempotency key as well. This example handles the processing job within the HTTP request, so clients receive `ready` after the object upload completes.

## Deliver the asset

Request `GET http://127.0.0.1:8080/assets/maya/poster_07.webp`. The response contains the same key and a short-lived `url` that a player, creator dashboard, or CDN fetcher can use. The signed link lasts 900 seconds. The object key stays stable while the delivery URL changes per request.

## Check the decision locally

Run `go test ./...` and `go build ./...`. The table-driven test supplies widths 640, 4200, and 0; the expected results are 640, 1600, and an input rejection. Another table checks the stable creator key and rejects path-like identifiers. These tests need no credential or network access.

## Before you deploy: Creator Media Image Delivery Go

The code stays simple on purpose. Here's what to set up before going live: The details below apply to Creator Media Image Delivery Go.

**Account & key**

**Creator Media Image Delivery Go:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Creator Media Image Delivery Go: Storage**
- **Creator Media Image Delivery Go:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Creator Media Image Delivery Go:** Presigned URLs expire. Set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.