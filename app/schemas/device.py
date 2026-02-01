from pydantic import BaseModel

class UpsertDeviceTokenRequest(BaseModel):
    user_id: str
    platform: str = "ios"
    fcm_token: str
