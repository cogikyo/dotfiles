# ewwd TODO

## Spotify Canvas (video album art)

Spotify has a "Canvas" feature — short looping videos (3-8s, 720x1280 MP4)
that play behind tracks in the mobile app. Not every track has one, but when
they do, we can show the video in the eww album art widget instead of a
static image.

### API details

- **Endpoint:** `POST https://spclient.wg.spotify.com/canvaz-cache/v0/canvases`
- **Format:** protobuf request/response (`application/x-protobuf`)
- **Request:** send `spotify:track:<id>` (available from playerctl metadata)
- **Response:** CDN URL like `https://canvaz.scdn.co/.../video/<id>.cnvs.mp4`
  - CDN links need no auth to download
  - Video: MP4 H.264, 720x1280 (9:16 vertical), 3-8 second loops

### Auth

Requires an internal Spotify access token (not the standard Web API OAuth).

1. Extract `sp_dc` cookie from `open.spotify.com` (browser DevTools → Cookies)
2. Store it in `configs/ewwd.yaml` under ewwd config (not a secret — it's a
   session cookie tied to your account, safe to keep in dotfiles)
3. Exchange for access token:
   ```
   GET https://open.spotify.com/get_access_token?reason=transport&productType=web_player
   Cookie: sp_dc=<value>
   ```
   Returns JSON with `accessToken` and `accessTokenExpirationTimestampMs`.
4. Token needs periodic refresh (check expiry, re-fetch as needed)

### Protobuf schema

```protobuf
syntax = "proto3";

message CanvasRequest {
  message Track {
    string track_uri = 1;
  }
  repeated Track tracks = 1;
}

message CanvasResponse {
  message Canvas {
    string id = 1;
    string canvas_url = 2;
    string track_uri = 5;
    message Artist {
      string artist_uri = 1;
      string artist_name = 2;
      string artist_img_url = 3;
    }
    Artist artist = 6;
    string other_id = 9;
    string canvas_uri = 11;
  }
  repeated Canvas canvases = 1;
}
```

Compile with: `protoc --go_out=. --go_opt=paths=source_relative canvas.proto`

### Implementation plan

1. **Add proto file** — `daemons/ewwd/providers/canvas.proto`, compile to Go
2. **Auth module** — fetch/cache/refresh access token from `sp_dc` cookie
3. **Canvas fetch in music.go** — on track change, query canvas endpoint
   - If canvas exists → download MP4 to `/tmp/eww/canvas.mp4`
   - Add `CanvasPath` and `HasCanvas` fields to `MusicState`
4. **Video overlay in eww** — use `mpv --loop --no-audio --no-osc` positioned
   over the album art area in the widget. Small but cool.
   - Fallback to album art when no canvas available
   - Kill mpv when track changes or playback stops

### Reference

- Go implementation: `shsf1382hAcKeR/Canvasify-API` on GitHub
- Python reference: `Delitefully/spotify-canvas-downloader`
- Spicetify extension: `itsmeow/Spicetify-Canvas`
