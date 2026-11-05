-- Origin SQL:
SELECT * EXCEPT STRICT timestamp FROM t;
SELECT * EXCEPT STRICT (timestamp, key) FROM t;


-- Beautify SQL:
SELECT
  * EXCEPT STRICT(timestamp)
FROM
  t;
SELECT
  * EXCEPT STRICT(timestamp, key)
FROM
  t;
