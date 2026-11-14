-- Se ejecuta una sola vez al crear el volumen de desarrollo (docker-entrypoint-initdb.d).
-- El rol de la aplicación no es superusuario ni dueño de las tablas, para que RLS se aplique.
CREATE ROLE app_user LOGIN PASSWORD 'app' NOSUPERUSER NOBYPASSRLS;

-- Tablas que cree el dueño (postgres, vía cmd/migrate) quedan con DML para app_user.
ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_user;
