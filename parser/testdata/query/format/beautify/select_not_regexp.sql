-- Origin SQL:
SELECT a, b FROM t WHERE name NOT REGEXP '^foo'


-- Beautify SQL:
SELECT
  a,
  b
FROM
  t
WHERE
  name NOT REGEXP '^foo';
