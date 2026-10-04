ALTER TABLE todos ADD COLUMN remind INTEGER NOT NULL DEFAULT 0 CHECK(remind IN (0,1));
CREATE INDEX reminder_task ON reminders(todo_id);
