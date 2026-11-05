-- Origin SQL:
SELECT avg(toUInt32OrNull(extract(arrayJoin(splitByString('//', s)) AS part, '(\d+)'))) AS v, part FROM t;
SELECT extract(s AS src, '\d+') AS n, src FROM t;
SELECT EXTRACT(YEAR FROM d AS day), day FROM t;
SELECT extract(s, '\d+'), EXTRACT(MONTH FROM d) FROM t;


-- Format SQL:
SELECT avg(toUInt32OrNull(EXTRACT(arrayJoin(splitByString('//', s)) AS part, '(\d+)'))) AS v, part FROM t;
SELECT EXTRACT(s AS src, '\d+') AS n, src FROM t;
SELECT EXTRACT(YEAR FROM d AS day), day FROM t;
SELECT EXTRACT(s, '\d+'), EXTRACT(MONTH FROM d) FROM t;
