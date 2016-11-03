-- +migrate Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE referentials (
  referential_id uuid PRIMARY KEY,
  slug           varchar(40) UNIQUE
);


-- +migrate Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE referentials;