-- Origin SQL:
SELECT * EXCEPT STRICT timestamp FROM t;
SELECT * EXCEPT STRICT (timestamp, key) FROM t;


-- Format SQL:
SELECT * EXCEPT STRICT(timestamp) FROM t;
SELECT * EXCEPT STRICT(timestamp, key) FROM t;
