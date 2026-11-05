-- Origin SQL:
SELECT db.t.* FROM db.t;
SELECT db.t.*, t.*, 1 FROM db.t AS t;
SELECT count(db.t.*) FROM db.t;
SELECT a.b.c FROM t;


-- Beautify SQL:
SELECT
  db.t.*
FROM
  db.t;
SELECT
  db.t.*,
  t.*,
  1
FROM
  db.t AS t;
SELECT
  count(db.t.*)
FROM
  db.t;
SELECT
  a.b.c
FROM
  t;
