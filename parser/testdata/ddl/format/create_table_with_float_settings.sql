-- Origin SQL:
CREATE TABLE t (a Int32) ENGINE = MergeTree ORDER BY a SETTINGS x = 0.5;


-- Format SQL:
CREATE TABLE t (a Int32) ENGINE = MergeTree ORDER BY a SETTINGS x=0.5;
