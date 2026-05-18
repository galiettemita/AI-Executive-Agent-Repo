-- Migration 007: UUIDv7 reconciliation
-- Redefines uuid_v7_generate() from gen_random_uuid() to real RFC 9562 UUIDv7.
-- Forward-only. Existing PKs remain valid; new PKs are time-ordered.
-- Decision rationale: Blueprint mandates UUIDv7 for time-ordering and monotonicity.

CREATE OR REPLACE FUNCTION uuid_v7_generate()
RETURNS uuid
LANGUAGE plpgsql
VOLATILE
AS $$
DECLARE
  unix_ts_ms bigint;
  uuid_bytes bytea;
BEGIN
  unix_ts_ms := (extract(epoch from clock_timestamp()) * 1000)::bigint;
  uuid_bytes := decode(lpad(to_hex(unix_ts_ms), 12, '0'), 'hex');  -- 6 bytes timestamp
  uuid_bytes := uuid_bytes || gen_random_bytes(10);                  -- 10 bytes random

  -- Set version 7 (bits 48-51 = 0111)
  uuid_bytes := set_byte(uuid_bytes, 6, (get_byte(uuid_bytes, 6) & x'0F'::int) | x'70'::int);

  -- Set variant 10xx (bits 64-65)
  uuid_bytes := set_byte(uuid_bytes, 8, (get_byte(uuid_bytes, 8) & x'3F'::int) | x'80'::int);

  RETURN encode(uuid_bytes, 'hex')::uuid;
END;
$$;

-- Verify the function produces valid UUIDv7 by checking version and variant bits
DO $$
DECLARE
  test_uuid uuid;
  hex_str text;
  version_char text;
  variant_nibble int;
BEGIN
  FOR i IN 1..100 LOOP
    test_uuid := uuid_v7_generate();
    hex_str := replace(test_uuid::text, '-', '');

    -- Check version nibble (position 13 in hex, 0-indexed = char 13) = '7'
    version_char := substring(hex_str from 13 for 1);
    IF version_char != '7' THEN
      RAISE EXCEPTION 'UUIDv7 version check failed: got %, expected 7', version_char;
    END IF;

    -- Check variant (position 17 in hex) high bits = 10xx (8,9,a,b)
    variant_nibble := ('x' || substring(hex_str from 17 for 1))::bit(4)::int;
    IF variant_nibble < 8 OR variant_nibble > 11 THEN
      RAISE EXCEPTION 'UUIDv7 variant check failed: got %, expected 8-11', variant_nibble;
    END IF;
  END LOOP;
  RAISE NOTICE 'UUIDv7 reconciliation validation passed: 100 UUIDs verified';
END;
$$;

-- Verify time-ordering: UUIDs generated in sequence should be ordered
DO $$
DECLARE
  prev_uuid uuid;
  curr_uuid uuid;
BEGIN
  prev_uuid := uuid_v7_generate();
  -- Small delay via pg_sleep to ensure different milliseconds
  PERFORM pg_sleep(0.002);
  FOR i IN 1..10 LOOP
    curr_uuid := uuid_v7_generate();
    IF curr_uuid::text < prev_uuid::text THEN
      RAISE EXCEPTION 'UUIDv7 ordering check failed: % should be > %', curr_uuid, prev_uuid;
    END IF;
    prev_uuid := curr_uuid;
    PERFORM pg_sleep(0.002);
  END LOOP;
  RAISE NOTICE 'UUIDv7 ordering validation passed: 10 sequential UUIDs verified monotonic';
END;
$$;
