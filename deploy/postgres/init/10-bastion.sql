-- Roles and databases of the local stack (P1-02 D4), run once as superuser by
-- the image's docker-entrypoint-initdb.d and by internal/platform/db/dbtest.
-- TD §1.4: databases bastion_pay and bastion_tickets.
-- TD §8.3 / §19: *_owner owns schemas, functions and runs migrations; *_app is
-- the application role. *_auditor and *_ops arrive with the items that use them.
-- Local only: each password is the role name (P1-02 D7).

CREATE ROLE pay_owner     LOGIN PASSWORD 'pay_owner';
CREATE ROLE pay_app       LOGIN PASSWORD 'pay_app';
CREATE ROLE tickets_owner LOGIN PASSWORD 'tickets_owner';
CREATE ROLE tickets_app   LOGIN PASSWORD 'tickets_app';

CREATE DATABASE bastion_pay     OWNER pay_owner;
CREATE DATABASE bastion_tickets OWNER tickets_owner;

-- One company's database per product (ARCH §7.1, §12.2): a product's roles
-- cannot connect to the other product's database (P1-02 D6).
REVOKE ALL ON DATABASE bastion_pay, bastion_tickets FROM PUBLIC;
GRANT CONNECT ON DATABASE bastion_pay     TO pay_app;
GRANT CONNECT ON DATABASE bastion_tickets TO tickets_app;
