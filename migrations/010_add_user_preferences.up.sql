BEGIN;

ALTER TABLE public.users
  ADD COLUMN IF NOT EXISTS preferences JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE OR REPLACE FUNCTION public.validate_user_preferences_json()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  v TEXT;
BEGIN
  IF NEW.preferences ? 'email_provider' THEN
    v := NEW.preferences->>'email_provider';
    IF v NOT IN ('google','microsoft','apple','imap','none') THEN
      RAISE EXCEPTION 'invalid preferences.email_provider: %', v;
    END IF;
  END IF;

  IF NEW.preferences ? 'music_provider' THEN
    v := NEW.preferences->>'music_provider';
    IF v NOT IN ('spotify','apple_music','youtube_music','none') THEN
      RAISE EXCEPTION 'invalid preferences.music_provider: %', v;
    END IF;
  END IF;

  IF NEW.preferences ? 'task_app' THEN
    v := NEW.preferences->>'task_app';
    IF v NOT IN ('todoist','things','ticktick','omnifocus','trello','asana','linear','jira','clickup','apple_reminders','none') THEN
      RAISE EXCEPTION 'invalid preferences.task_app: %', v;
    END IF;
  END IF;

  IF NEW.preferences ? 'notes_app' THEN
    v := NEW.preferences->>'notes_app';
    IF v NOT IN ('apple_notes','notion','bear','obsidian','craft','google_keep','reflect','none') THEN
      RAISE EXCEPTION 'invalid preferences.notes_app: %', v;
    END IF;
  END IF;

  IF NEW.preferences ? 'finance_app' THEN
    v := NEW.preferences->>'finance_app';
    IF v NOT IN ('ynab','monarch','copilot','none') THEN
      RAISE EXCEPTION 'invalid preferences.finance_app: %', v;
    END IF;
  END IF;

  IF NEW.preferences ? 'has_edge_agent' THEN
    v := NEW.preferences->>'has_edge_agent';
    IF v NOT IN ('true','false') THEN
      RAISE EXCEPTION 'invalid preferences.has_edge_agent: %', v;
    END IF;
  END IF;

  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_users_validate_preferences ON public.users;
CREATE TRIGGER trg_users_validate_preferences
BEFORE INSERT OR UPDATE ON public.users
FOR EACH ROW
EXECUTE FUNCTION public.validate_user_preferences_json();

COMMIT;
