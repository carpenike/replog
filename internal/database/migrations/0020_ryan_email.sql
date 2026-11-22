-- +goose Up

-- Backfill one address. users.email already exists (UNIQUE COLLATE NOCASE)
-- and stays admin-editable. Set ryan@ryanholt.net only when a user named
-- ryan exists, that row's email is empty, and the address is not already
-- taken. Do not insert a user and do not touch any other row.

UPDATE users
SET email = 'ryan@ryanholt.net'
WHERE id = (
    SELECT id FROM users
    WHERE username = 'ryan'
      AND (email IS NULL OR TRIM(email) = '')
      AND NOT EXISTS (
          SELECT 1 FROM users WHERE email = 'ryan@ryanholt.net'
      )
);

-- +goose Down

UPDATE users
SET email = NULL
WHERE username = 'ryan'
  AND email = 'ryan@ryanholt.net';
