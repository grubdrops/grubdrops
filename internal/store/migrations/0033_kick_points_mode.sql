-- +goose Up
-- Kick accounts gain a points mode. Before, a Kick account ran
-- drops_points whatever was stored, and the stored default was off, so
-- existing Kick accounts keep their behaviour as drops_points.
UPDATE accounts SET points_mode = 'drops_points' WHERE platform = 'kick' AND points_mode = 'off';

-- +goose Down
SELECT 1;
