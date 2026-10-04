CREATE TABLE todos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL CHECK(length(trim(title)) BETWEEN 1 AND 500),
    description TEXT NOT NULL DEFAULT '',
    completed INTEGER NOT NULL DEFAULT 0 CHECK(completed IN (0, 1)),
    completed_at INTEGER,
    due_at INTEGER,
    snoozed_until INTEGER,
    priority INTEGER NOT NULL DEFAULT 0 CHECK(priority BETWEEN 0 AND 3),
    temporary INTEGER NOT NULL DEFAULT 0 CHECK(temporary IN (0, 1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK((completed = 0 AND completed_at IS NULL) OR (completed = 1 AND completed_at IS NOT NULL))
);
CREATE TABLE desktop_presentations (
    todo_id INTEGER PRIMARY KEY REFERENCES todos(id) ON DELETE CASCADE,
    mode TEXT NOT NULL CHECK(mode IN ('NONE', 'EDGE')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE TABLE edge_stacks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    display_id TEXT NOT NULL DEFAULT '',
    side TEXT NOT NULL CHECK(side IN ('left', 'right')),
    offset REAL NOT NULL CHECK(offset BETWEEN 0 AND 1),
    density TEXT NOT NULL CHECK(density IN ('compact', 'normal', 'relaxed')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(display_id, side)
);
CREATE TABLE edge_stack_items (
    stack_id INTEGER NOT NULL REFERENCES edge_stacks(id) ON DELETE CASCADE,
    todo_id INTEGER NOT NULL UNIQUE REFERENCES desktop_presentations(todo_id) ON DELETE CASCADE,
    sort_order INTEGER NOT NULL,
    PRIMARY KEY(stack_id, todo_id)
);
CREATE INDEX edge_stack_order ON edge_stack_items(stack_id, sort_order, todo_id);
CREATE TABLE reminders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    todo_id INTEGER NOT NULL REFERENCES todos(id) ON DELETE CASCADE,
    remind_at INTEGER NOT NULL,
    delivered_at INTEGER,
    created_at INTEGER NOT NULL
);
