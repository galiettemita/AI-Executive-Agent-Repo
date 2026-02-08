# Executive AI Agent — Companion App (Foundation)

This is the React Native (Expo) companion app foundation. It pairs with the backend using a one‑time pairing code and then uses JWTs for authenticated API calls.

## 1) Set the API base URL
Create a `.env` file in this folder:
```
EXPO_PUBLIC_API_BASE_URL=https://your-backend.onrender.com
```

## 2) Install and run
```
npm install
npm run start
```

## 3) Create a pairing code (admin step)
From your terminal, call the backend admin endpoint (requires HMAC headers).

```
export BASE_URL="https://your-backend.onrender.com"
export USER_ID="YOUR_REAL_USER_ID"
export STATE_SIGNING_SECRET="PASTE_STATE_SIGNING_SECRET_HERE"
export TS=$(date +%s)

export SIG=$(python3 - <<'PY'
import base64, hmac, hashlib, os
user_id = os.environ["USER_ID"]
ts = os.environ["TS"]
secret = os.environ["STATE_SIGNING_SECRET"]
msg = f"{user_id}.{ts}".encode("utf-8")
sig = base64.urlsafe_b64encode(hmac.new(secret.encode(), msg, hashlib.sha256).digest()).decode().rstrip("=")
print(sig)
PY
)

curl -X POST "$BASE_URL/admin/pairing/code" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: $USER_ID" \
  -H "X-User-Timestamp: $TS" \
  -H "X-User-Signature: $SIG" \
  -d '{"user_id":"'"$USER_ID"'"}'
```

The response includes a short pairing code. Enter that code in the mobile app.

## 4) Pair in the app
Open the app, paste the pairing code, and tap **Connect**. The app will store the JWT and user id locally.

## Notes
- Pairing code TTL is controlled by `PAIRING_CODE_TTL_MINUTES` on the backend.
- JWT lifetime is controlled by `JWT_ACCESS_TTL_HOURS` on the backend.
