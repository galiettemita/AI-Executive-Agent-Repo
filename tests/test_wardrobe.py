from fastapi.testclient import TestClient

from app.main import app


def test_wardrobe_item_crud_and_photos():
    client = TestClient(app)
    user_id = "wardrobe_user_1"

    create_payload = {
        "user_id": user_id,
        "name": "Black Jacket",
        "category": "outerwear",
        "color": "black",
        "tags": ["jacket", "winter"],
    }
    resp = client.post("/wardrobe/items", json=create_payload)
    assert resp.status_code == 200
    item = resp.json()["item"]
    item_id = item["id"]

    resp = client.get("/wardrobe/items", params={"user_id": user_id})
    assert resp.status_code == 200
    assert any(i["id"] == item_id for i in resp.json()["items"])

    files = {"file": ("jacket.jpg", b"abc123", "image/jpeg")}
    data = {"user_id": user_id, "primary": "true"}
    resp = client.post(f"/wardrobe/items/{item_id}/photos/upload", data=data, files=files)
    assert resp.status_code == 200
    photo_id = resp.json()["photo_id"]
    assert photo_id

    resp = client.get(f"/wardrobe/items/{item_id}/photos", params={"user_id": user_id})
    assert resp.status_code == 200
    photos = resp.json()["photos"]
    assert any(p["id"] == photo_id for p in photos)

    resp = client.patch(
        f"/wardrobe/items/{item_id}",
        json={"user_id": user_id, "color": "matte black"},
    )
    assert resp.status_code == 200
    assert resp.json()["item"]["color"] == "matte black"

    resp = client.delete(f"/wardrobe/items/{item_id}", params={"user_id": user_id})
    assert resp.status_code == 200
