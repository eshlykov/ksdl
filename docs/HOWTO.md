# How to Download a Kinescope Video

This guide walks you through downloading a video from Kinescope. It is tailored to the macOS environment where this was first performed.

---

## Prerequisites

All tools below are already installed on this machine. No installation needed.

| Tool | Location | Purpose |
|------|----------|---------|
| Go | `go` | Runs the automated download script |
| N_m3u8DL-RE | `/usr/local/bin/N_m3u8DL-RE` | Downloads HLS stream segments |
| FFmpeg | `ffmpeg` | Merges video and audio into final MP4 |
| yt-dlp | `/opt/homebrew/bin/yt-dlp` | Downloads and decrypts the video stream |
| mp4decrypt | `/opt/homebrew/bin/mp4decrypt` | Decrypts the encrypted audio stream |

> **Note:** N_m3u8DL-RE was unblocked from macOS Gatekeeper with this one-time command (already done):
> ```bash
> xattr -d com.apple.quarantine /usr/local/bin/N_m3u8DL-RE
> ```

---

## What's Going On (brief overview)

Kinescope encrypts its video and audio streams using AES. The encryption key is freely available from their license server — no login needed. The challenge is that standard tools can't decrypt it automatically, so we do it in steps:

1. Find the stream URL in Chrome DevTools
2. Download the encrypted segments (video + audio) with N_m3u8DL-RE
3. Grab the decryption key from the browser
4. Re-download the video (now decrypted) with `yt-dlp`
5. Decrypt the audio with `mp4decrypt`
6. Merge both into one MP4 with `ffmpeg`

---

## Step 1 — Open the Video in Chrome and Open DevTools

1. Navigate to the page that contains the embedded video.
2. Press **Cmd+Option+I** to open Chrome DevTools.
3. Click the **Network** tab.
4. Play the video (or reload the page if it doesn't start).

---

## Step 2 — Find the Embed URL and Referrer

In the Network tab, look for a request with these properties:
- **Domain:** `kinescope.io`
- **Method:** `GET`
- **Status:** `200`
- **Type:** `document`
- The path looks like `/AbCdEfGhIjKlMnO` (a short alphanumeric ID)

Click that request. You need two things:

**A. The Embed URL** — the full Request URL, e.g.:
```
https://kinescope.io/q4WzE3saqokSWORJuTxKiE
```

**B. The Referrer** — look in the Request Headers for the `referer` field, e.g.:
```
https://example.com/
```
(This is also just the URL of the page you are currently on — you can copy it from the browser address bar.)

Write both down — you'll need them in the next step.

---

## Step 3 — Find the m3u8 URL and Download Segments

**A. Find the m3u8 URL in DevTools**

Still in the Network tab, type `m3u8` into the filter box. Look for a request with:
- **Domain:** `kinescope.io`
- **Path:** something like `/<uuid>/master.m3u8?expires=...&sign=...`

Click that request and copy the full **Request URL** — this is your m3u8 link. Example:

```
https://kinescope.io/966fcd22-53a3-4c51-8ebe-52ee3ef37bfc/master.m3u8?expires=...&sign=...
```

> **Important:** This URL expires. Complete the download in the same browser session.

**B. Download the segments**

Open Terminal and navigate to the working directory:

```bash
cd /Users/zheka/kinescope
```

Run N_m3u8DL-RE with the m3u8 URL you just copied and the referrer from Step 2B:

```bash
N_m3u8DL-RE \
  -H "referer: https://example.com/" \
  --log-level INFO \
  --del-after-done \
  -M format=mp4:muxer=ffmpeg \
  --save-name "video" \
  --auto-select \
  "https://kinescope.io/966fcd22-53a3-4c51-8ebe-52ee3ef37bfc/master.m3u8?expires=...&sign=..."
```

N_m3u8DL-RE will download 100% of both video and audio, then **fail at the muxing step** — that is expected. You'll see something like:

```
ERROR: Mux failed
ERROR: Failed
```

This is fine. Two files were created — note their names from the N_m3u8DL-RE output printed just before the error, e.g.:

```
WARN : video.mp4          ← encrypted video
WARN : video.und.m4a      ← encrypted audio
```

Write down the **encrypted audio filename** (the `.und.m4a` one). You'll need it in Step 7.

---

## Step 4 — Get the Decryption Key from the Browser

Go back to Chrome DevTools Network tab. Find the request with:
- **Domain:** `license.kinescope.io`
- **Method:** `GET`
- **Status:** `200`

Click that request.

**A. Get the KID** from the Request URL path. It's the long hex string between `sample-aes/` and `?token=`:

```
/v1/vod/.../acquire/sample-aes/87a5f6449f218340af79ae61c8895f69?token=
                                ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
                                This is the KID — copy it
```

**B. Get the Key** — click the **Response** tab of that request. You'll see a short string, e.g.:

```
/7zYlEOn/MuHLN4=
```

Copy that string exactly, including any trailing `=` characters.

---

## Step 5 — Convert the Key to Hex

The decryption tool requires the key as a hex string. Run this command in Terminal, replacing the key string with the one you copied:

```bash
echo -n "/7zYlEOn/MuHLN4=" | xxd -p
```

It will output a 32-character hex string like:

```
2f377a596c454f6e2f4d75484c4e343d
```

Copy that hex string — this is your **KEY in hex**. You now have:
- **KID** (from Step 4A): e.g. `87a5f6449f218340af79ae61c8895f69`
- **KEY** (hex, from above): e.g. `2f377a596c454f6e2f4d75484c4e343d`

---

## Step 6 — Download the Decrypted Video

Run yt-dlp with the m3u8 URL you copied in Step 3. Replace the URL and referrer with your own values:

```bash
yt-dlp \
  --referer "https://example.com/" \
  --add-header "Origin:https://kinescope.io" \
  -f "bestvideo+bestaudio" \
  --merge-output-format mp4 \
  "https://kinescope.io/966fcd22-53a3-4c51-8ebe-52ee3ef37bfc/master.m3u8?expires=...&sign=..."
```

> Replace the `--referer` value with your referrer from Step 2B, and the URL at the end with your m3u8 URL from Step 3.

yt-dlp will delegate to ffmpeg and download only the video track (audio will fail — that's expected). The output file will be named something like:

```
master.f6713.mp4
```

The number (`6713`) is the video bitrate and will vary. Note the exact filename.

---

## Step 7 — Decrypt the Audio

Run mp4decrypt on the encrypted audio file from Step 3. Replace the KID, KEY, and filenames with your values:

```bash
mp4decrypt \
  --key 87a5f6449f218340af79ae61c8895f69:2f377a596c454f6e2f4d75484c4e343d \
  "Sample video.mov.und.m4a" \
  "audio_decrypted.m4a"
```

- After `--key`, the format is `KID:KEY` (both in hex, joined by a colon)
- The second argument is the **encrypted audio filename** (from Step 3)
- The third argument is the output name — you can use any name ending in `.m4a`

If successful, the command produces no output. A new file `audio_decrypted.m4a` appears in the directory.

---

## Step 8 — Merge Video and Audio into the Final File

Run ffmpeg to combine both streams. Replace the filenames with your actual values:

```bash
ffmpeg \
  -i "master.f6713.mp4" \
  -i "audio_decrypted.m4a" \
  -c copy \
  "final_output.mp4"
```

- `-i "master.f6713.mp4"` — the decrypted video from Step 6
- `-i "audio_decrypted.m4a"` — the decrypted audio from Step 7
- `-c copy` — copy streams without re-encoding (fast)
- `"final_output.mp4"` — the name for your final file (rename as you like)

ffmpeg will finish in a few seconds. Your video is ready.

---

## Verify the Result (optional)

To confirm the final file has both video and audio:

```bash
ffprobe -v error -show_streams -of compact "final_output.mp4" 2>&1 | grep codec_type
```

You should see one line with `codec_type=video` and one with `codec_type=audio`.

---

## Files in the Directory After Completion

| File | What it is |
|------|-----------|
| `<title>.mp4` | Encrypted video (from N_m3u8DL-RE) |
| `<title>.und.m4a` | Encrypted audio (from N_m3u8DL-RE) |
| `master.f<bitrate>.mp4` | Decrypted video (from yt-dlp) |
| `audio_decrypted.m4a` | Decrypted audio (from mp4decrypt) |
| `final_output.mp4` | **Your final video** |

Keep or delete the intermediate files as you see fit.
