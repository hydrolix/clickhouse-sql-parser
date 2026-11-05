-- Origin SQL:
SELECT t.* EXCEPT a FROM t;
SELECT t.* EXCEPT STRICT (a, b) APPLY(sum) FROM t;


-- Beautify SQL:
SELECT
  t.* EXCEPT(a)
FROM
  t;
SELECT
  t.* EXCEPT STRICT(a, b) APPLY(sum)
FROM
  t;
