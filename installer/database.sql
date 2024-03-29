CREATE DATABASE IF NOT EXISTS vaccinator;
USE vaccinator;

CREATE TABLE IF NOT EXISTS data (
  VID BYTES NOT NULL,
  PAYLOAD BYTES NOT NULL,
  PROVIDERID SMALLINT NOT NULL,
  CREATIONDATE TIMESTAMPTZ NOT NULL,
  DURATION SMALLINT NOT NULL DEFAULT 0,
  PRIMARY KEY (VID)
);

CREATE TABLE IF NOT EXISTS provider (
  PROVIDERID SMALLINT NOT NULL,
  NAME STRING NOT NULL,
  DESCRIPTION STRING NOT NULL DEFAULT '',
  PASSWORD STRING NOT NULL,
  IP STRING NOT NULL DEFAULT '',
  CREATIONDATE TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (PROVIDERID)
);

CREATE TABLE IF NOT EXISTS search (
  VID BYTES NOT NULL,
  WORD STRING NOT NULL,
  INDEX (WORD),
  INDEX (VID)
);

CREATE TABLE IF NOT EXISTS audit (
  ID SERIAL,
  LOGTYPE INT NOT NULL,
  LOGDATE TIMESTAMPTZ NOT NULL,
  PROVIDERID SMALLINT NOT NULL,
  LOGCOMMENT STRING DEFAULT '',
  PRIMARY KEY (ID)
);

CREATE TABLE IF NOT EXISTS nodes (
  NODEID INT NOT NULL,
  LASTACTIVITY TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (NODEID)
);

INSERT INTO provider (providerid, name, password, ip, creationdate) 
  VALUES(1, 'test', 'vaccinator', '127.0.0.1', now())
ON CONFLICT DO NOTHING;

CREATE USER IF NOT EXISTS <USER>;
GRANT ALL ON TABLE vaccinator.* TO <USER> WITH GRANT OPTION;