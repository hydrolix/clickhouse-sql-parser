-- Origin SQL:
SELECT * REPLACE(i + 1 AS i) EXCEPT colX APPLY(sum) FROM t


-- Format SQL:
SELECT * REPLACE(i + 1 AS i) EXCEPT(colX) APPLY(sum) FROM t;
