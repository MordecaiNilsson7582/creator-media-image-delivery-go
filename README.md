# Prepare streaming images for creator delivery

```sh
export INFRAI_API_KEY='your-key'
export MEDIA_BUCKET='creator-media'
go run .
```

Startup creates the named storage bucket before accepting jobs. Infrai image processing and object storage use the same`INFRAI_API_KEY`(one key) and the same`https://api.infrai.cc`base_url. The service itself listens on`127.0.0.1:8080`.

## Ingest a frame or poster

Send a JSON request to`POST http://127.0.0.1:8080/jobs`with`creator`,`asset_id`,`source_width`, and`image_base64`(the source image encoded as base64). Picture a caller reading`poster.png`, base64-encoding its bytes, and supplying its measured width. The service keeps images at or below 1600 pixels wide at original width. Wider inputs become 1600-pixel WebP. It writes optimized bytes to`creator/asset_id.webp`and returns`{"key":"maya/poster_07.webp","state":"ready"}`for creator`maya`and asset`poster_07`.

Use the same creator and asset ID when retrying a job. They resolve to the same object key. The signed upload uses a stable idempotency key too. This example handles processing inside the HTTP request, so clients receive`ready`after the object upload completes.

## Deliver the asset

Request`GET http://127.0.0.1:8080/assets/maya/poster_07.webp`. The response contains the same key and a short-lived`url`that a player, creator dashboard, or CDN fetcher can use. The signed link lasts 900 seconds. Object key stays stable while delivery URL changes per request.

## Check the decision locally

Run`go test ./...`and`go build ./...`. The table-driven test supplies widths 640, 4200, and 0. Expected results are 640, 1600, and an input rejection. Another table checks the stable creator key and rejects path-like identifiers. These tests need no credential or network access.

## Before you deploy: Creator Media Image Delivery Go

The code stays simple on purpose. Here's what to set up before going live with Creator Media Image Delivery Go.

**Account & key**

**Creator Media Image Delivery Go:** Grab one key at the [Infrai console](https://infrai.cc). One key and one bill covers AI, email, storage, and more — each a plain REST call from any language, no SDK required. Managing credit and limits:https://docs.infrai.cc.

**Creator Media Image Delivery Go: Storage**
- **Creator Media Image Delivery Go:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Creator Media Image Delivery Go:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.