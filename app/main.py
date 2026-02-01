# starts FASTAPI + includes routes
from fastapi import FastAPI
from app.db.database import Base, engine
import app.db.models
from app.api.routes.health import router as health_router
from app.api.routes.chat import router as chat_router
from app.api.routes.device import router as device_router
from app.api.routes.watch import router as watch_router
from app.api.routes.discover import router as discover_router
from app.api.routes.assist import router as assist_router
from app.api.routes.watch_refresh import router as watch_refresh_router
from dotenv import load_dotenv
from app.api.routes.notifications import router as notifications_router
from app.api.routes.agent_chat import router as agent_chat_router
from app.api.routes import webhooks_whatsapp
from app.api.routes.admin_google import router as admin_google_router
from app.api.routes.admin_tasks import router as admin_tasks_router

load_dotenv()



# Simple MVP table creation; later replace with migrations
Base.metadata.create_all(bind=engine)

app = FastAPI(title="Shopping Assistant Backend", version="0.1.0")

app.include_router(health_router, tags=["health"])
app.include_router(chat_router, prefix="/chat", tags=["chat"])
app.include_router(device_router, prefix="/device", tags=["device"])
app.include_router(watch_router, prefix="/watch", tags=["watch"])
app.include_router(discover_router, prefix="/discover", tags=["discover"])
app.include_router(assist_router, prefix="/assist", tags=["assist"])
app.include_router(watch_refresh_router)
app.include_router(notifications_router)
app.include_router(agent_chat_router)
app.include_router(webhooks_whatsapp.router)
app.include_router(admin_google_router)
app.include_router(admin_tasks_router)