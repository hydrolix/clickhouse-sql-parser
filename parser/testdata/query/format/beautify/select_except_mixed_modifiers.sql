-- Origin SQL:
SELECT * REPLACE(i + 1 AS i) EXCEPT colX APPLY(sum) FROM t


-- Beautify SQL:
SELECT
  * REPLACE(i + 1 AS i) EXCEPT(colX) APPLY(sum)
FROM
  t;
