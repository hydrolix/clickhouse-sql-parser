-- Origin SQL:
SELECT t.* EXCEPT a FROM t;
SELECT t.* EXCEPT STRICT (a, b) APPLY(sum) FROM t;


-- Format SQL:
SELECT t.* EXCEPT(a) FROM t;
SELECT t.* EXCEPT STRICT(a, b) APPLY(sum) FROM t;
