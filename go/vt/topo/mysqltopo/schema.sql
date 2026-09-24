-- Schema for the MySQL topology server.
--
-- This file is the single source of truth for the topo tables. It is embedded
-- into the binary and applied by CreateSchema (see server.go), and is also
-- applied directly by cluster bootstrap scripts such as
-- examples/common/scripts/mysql-up.sh, which have no way to call into Go.
-- Keep it idempotent: both callers may run it against an existing topology.
--
-- Statements are separated by a line containing only a semicolon.

CREATE TABLE IF NOT EXISTS topo_data (
	path VARCHAR(512) NOT NULL PRIMARY KEY,
	data MEDIUMBLOB,
	version BIGINT NOT NULL DEFAULT 1,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	modified_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB
;

CREATE TABLE IF NOT EXISTS topo_locks (
	path VARCHAR(512) NOT NULL PRIMARY KEY,
	contents TEXT,
	expires_at TIMESTAMP NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	INDEX expires_idx (expires_at)
) ENGINE=InnoDB
;

CREATE TABLE IF NOT EXISTS topo_elections (
	name VARCHAR(512) NOT NULL PRIMARY KEY,
	leader_id VARCHAR(255) NOT NULL,
	contents TEXT,
	expires_at TIMESTAMP NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	modified_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	INDEX expires_idx (expires_at)
) ENGINE=InnoDB
;
