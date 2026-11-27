DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'api_app') THEN
    CREATE ROLE api_app LOGIN NOINHERIT PASSWORD 'api_app_password';
  ELSE
    ALTER ROLE api_app WITH LOGIN NOINHERIT PASSWORD 'api_app_password';
  END IF;
END $$;