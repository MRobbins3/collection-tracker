-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE users (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    google_sub   text NOT NULL UNIQUE,
    email        text NOT NULL,
    display_name text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE categories (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug             text NOT NULL UNIQUE,
    name             text NOT NULL,
    description      text,
    attribute_schema jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX categories_name_trgm ON categories USING GIN (name gin_trgm_ops);

CREATE TABLE collections (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id uuid NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    name        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX collections_user_idx ON collections(user_id);

CREATE TABLE items (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_id uuid NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    name          text NOT NULL,
    quantity      integer NOT NULL DEFAULT 1 CHECK (quantity >= 0),
    condition     text,
    attributes    jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX items_collection_idx ON items(collection_id);
CREATE INDEX items_attributes_gin ON items USING GIN (attributes jsonb_path_ops);
CREATE INDEX items_name_trgm       ON items USING GIN (name gin_trgm_ops);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS collections;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
