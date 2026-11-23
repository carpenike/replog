-- +goose Up

-- Retired. This migration once backfilled one user's email address. It
-- has already run where it was needed, and the address does not belong
-- in a public repository, so it is now a no-op. Admins set user email
-- addresses in the app (users.email stays admin-editable).
SELECT 1;

-- +goose Down

SELECT 1;
