from app.core.celery_app import celery_app


def test_celery_app_importable():
    assert celery_app.main == "executive_ai_agent"
    assert celery_app.conf.task_track_started is True
