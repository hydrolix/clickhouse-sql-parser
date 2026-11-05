-- Origin SQL:
SELECT toStartOfInterval(ts, INTERVAL 1 MINUTE) AS interval, count() FROM t GROUP BY interval ORDER BY interval;
SELECT interval FROM t WHERE interval > 0 ORDER BY interval DESC;
SELECT (interval), interval + 1 FROM t;
SELECT now() - INTERVAL 1 HOUR, INTERVAL (1) MINUTE, INTERVAL x DAY FROM t;


-- Format SQL:
SELECT toStartOfInterval(ts, INTERVAL 1 MINUTE) AS interval, count() FROM t GROUP BY interval ORDER BY interval;
SELECT interval FROM t WHERE interval > 0 ORDER BY interval DESC;
SELECT (interval), interval + 1 FROM t;
SELECT now() - INTERVAL 1 HOUR, INTERVAL (1) MINUTE, INTERVAL x DAY FROM t;
