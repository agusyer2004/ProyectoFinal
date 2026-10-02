#!/bin/sh
# Corre una sola vez, al inicializar el volumen. Crea un usuario por servicio con el mínimo de permisos.
# El dueño de las tablas es POSTGRES_USER (el que aplica las migraciones). Los permisos sobre tablas
# que todavía no existen se dan con ALTER DEFAULT PRIVILEGES, así las migraciones del Bloque 3 no
# tienen que repetirlos.
set -eu

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  -v sink_password="$SINK_DB_PASSWORD" -v panel_password="$PANEL_DB_PASSWORD" <<'SQL'
-- sink: escribe eventos, alertas y dlq. Lee events para el chequeo de idempotencia y las pruebas.
CREATE ROLE sink_user LOGIN PASSWORD :'sink_password';
-- panel: solo lectura.
CREATE ROLE panel_user LOGIN PASSWORD :'panel_password';

REVOKE ALL ON DATABASE :"DBNAME" FROM PUBLIC;
GRANT CONNECT ON DATABASE :"DBNAME" TO sink_user, panel_user;
GRANT USAGE ON SCHEMA public TO sink_user, panel_user;

ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT ON TABLES TO sink_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO panel_user;
SQL
