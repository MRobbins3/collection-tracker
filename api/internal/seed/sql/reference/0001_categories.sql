-- Reference seed: the curated category list and each category's attribute
-- schema (JSON Schema rendered as the add/edit item form and validated
-- server-side). Applied on every API start, in every environment.
--
-- Idempotent: keyed on slug. Re-running refreshes name/description/schema
-- in place (ids stay stable, so collections keep pointing at them) and is a
-- no-op when nothing changed. Every schema property needs a human "title"
-- (see Documentation/categories/README.md).
--
-- catalog_identifier names the attribute that, with the name, tells two
-- catalog entries apart (set number, ISBN, artist...); NULL means the name
-- alone does. It must be one of the schema's properties.
--
-- Removing a category from this file does NOT delete it from the database;
-- write a migration for that so the collections that reference it are
-- handled deliberately.

INSERT INTO categories (slug, name, description, attribute_schema, catalog_identifier)
SELECT v.slug, v.name, v.description, v.attribute_schema, v.catalog_identifier
FROM (VALUES
(
    'lego-sets',
    'Lego Sets',
    'Official Lego sets, minifigures, and themed collections.',
    '{
      "type": "object",
      "properties": {
        "set_number": {
          "type": "string",
          "title": "Set number",
          "description": "The number printed on the box, e.g. 75192."
        },
        "theme": {
          "type": "string",
          "title": "Theme",
          "description": "Star Wars, Technic, Creator Expert, etc."
        },
        "piece_count": {
          "type": "integer",
          "minimum": 0,
          "title": "Piece count",
          "description": "Total number of pieces in the set."
        }
      },
      "additionalProperties": false
    }'::jsonb,
    'set_number'
),
(
    'funko-pops',
    'Funko Pops',
    'Funko Pop! vinyl figures, chases, and exclusives.',
    '{
      "type": "object",
      "properties": {
        "series": {
          "type": "string",
          "title": "Series",
          "description": "The Pop series, e.g. Marvel, Disney, Star Wars."
        },
        "number": {
          "type": "integer",
          "minimum": 0,
          "title": "Pop number",
          "description": "The figure''s number within its series."
        },
        "variant": {
          "type": "string",
          "title": "Variant",
          "description": "Chase, exclusive, glow-in-the-dark, etc."
        }
      },
      "additionalProperties": false
    }'::jsonb,
    'number'
),
(
    'coins',
    'Coins',
    'Circulating and commemorative coins from around the world.',
    '{
      "type": "object",
      "properties": {
        "year": {
          "type": "integer",
          "title": "Year"
        },
        "country": {
          "type": "string",
          "title": "Country"
        },
        "denomination": {
          "type": "string",
          "title": "Denomination",
          "description": "Face value, e.g. 1 cent, 50 pence."
        },
        "mint_mark": {
          "type": "string",
          "title": "Mint mark",
          "description": "Which mint produced the coin (e.g. D, P, S)."
        }
      },
      "additionalProperties": false
    }'::jsonb,
    NULL
),
(
    'stamps',
    'Stamps',
    'Postal stamps, commemoratives, and first-day covers.',
    '{
      "type": "object",
      "properties": {
        "year": {
          "type": "integer",
          "title": "Year"
        },
        "country": {
          "type": "string",
          "title": "Country"
        },
        "catalog_id": {
          "type": "string",
          "title": "Catalog ID",
          "description": "Scott, Stanley Gibbons, Michel, or similar reference."
        }
      },
      "additionalProperties": false
    }'::jsonb,
    'catalog_id'
),
(
    'trading-cards',
    'Trading Cards',
    'Sports cards, TCGs, and collectible card games.',
    '{
      "type": "object",
      "properties": {
        "game": {
          "type": "string",
          "title": "Game",
          "description": "Magic, Pokémon, NBA, Baseball, etc."
        },
        "set_name": {
          "type": "string",
          "title": "Set name",
          "description": "The expansion or set, e.g. Modern Horizons 2, 2024 Topps Series 1."
        },
        "card_number": {
          "type": "string",
          "title": "Card number"
        },
        "rarity": {
          "type": "string",
          "title": "Rarity",
          "description": "Common, uncommon, rare, mythic, etc."
        }
      },
      "additionalProperties": false
    }'::jsonb,
    'set_name'
),
(
    'plants',
    'Plants',
    'Houseplants, succulents, and garden specimens.',
    '{
      "type": "object",
      "properties": {
        "species": {
          "type": "string",
          "title": "Species",
          "description": "Common or scientific name."
        },
        "pot_size": {
          "type": "string",
          "title": "Pot size",
          "description": "e.g. 4 inch, 1 gallon, bonsai tray."
        },
        "acquired_from": {
          "type": "string",
          "title": "Acquired from",
          "description": "Nursery, friend, propagation, etc."
        }
      },
      "additionalProperties": false
    }'::jsonb,
    NULL
),
(
    'books',
    'Books',
    'Physical books — fiction, non-fiction, first editions, signed copies.',
    '{
      "type": "object",
      "properties": {
        "author": {
          "type": "string",
          "title": "Author"
        },
        "isbn": {
          "type": "string",
          "title": "ISBN",
          "description": "13-digit or 10-digit ISBN."
        },
        "edition": {
          "type": "string",
          "title": "Edition",
          "description": "First, revised, signed, etc."
        }
      },
      "additionalProperties": false
    }'::jsonb,
    'isbn'
),
(
    'vinyl-records',
    'Vinyl Records',
    'LPs, EPs, 7-inches, and box sets.',
    '{
      "type": "object",
      "properties": {
        "artist": {
          "type": "string",
          "title": "Artist"
        },
        "album": {
          "type": "string",
          "title": "Album"
        },
        "year": {
          "type": "integer",
          "title": "Year"
        },
        "label": {
          "type": "string",
          "title": "Label"
        },
        "pressing": {
          "type": "string",
          "title": "Pressing",
          "description": "Original, reissue, colored vinyl, etc."
        }
      },
      "additionalProperties": false
    }'::jsonb,
    'artist'
)) AS v (slug, name, description, attribute_schema, catalog_identifier)
-- A user-built category approved under the same name (ADR 0013) keeps it:
-- the system category is skipped rather than failing the boot on the
-- one-approved-category-per-name index. Seed names are plain ASCII, so
-- this lower/trim/collapse matches categories.name_key.
WHERE NOT EXISTS (
    SELECT 1 FROM categories c
    WHERE c.status = 'approved' AND c.slug <> v.slug
      AND c.name_key = lower(regexp_replace(btrim(v.name), '\s+', ' ', 'g'))
)
ON CONFLICT (slug) DO UPDATE SET
    name               = EXCLUDED.name,
    description        = EXCLUDED.description,
    attribute_schema   = EXCLUDED.attribute_schema,
    catalog_identifier = EXCLUDED.catalog_identifier
WHERE (categories.name, categories.description, categories.attribute_schema, categories.catalog_identifier)
    IS DISTINCT FROM (EXCLUDED.name, EXCLUDED.description, EXCLUDED.attribute_schema, EXCLUDED.catalog_identifier);
